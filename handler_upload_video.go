package main

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
	const upload_limit = 1 << 30
	r.Body = http.MaxBytesReader(w, r.Body, upload_limit)

	videoIDString := r.PathValue("videoID")
	videoID, err := uuid.Parse(videoIDString)
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Invalid ID", err)
		return
	}
	token, err := auth.GetBearerToken(r.Header)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't find JWT", err)
		return
	}

	userID, err := auth.ValidateJWT(token, cfg.jwtSecret)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Couldn't validate JWT", err)
		return
	}

	video, err := cfg.db.GetVideo(videoID)
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "Could not get Video metadata", err)
		return
	}
	if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "video is not owned by user", err)
		return
	}
	file, header, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Could not form file", err)
		return
	}
	defer file.Close()

	media_type, _, err := mime.ParseMediaType(header.Header.Get("Content-Type"))
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Could not parse media type", err)
		return
	}
	if media_type != "video/mp4" {
		respondWithError(w, http.StatusBadRequest, "Unauthorized file type", nil)
		return
	}

	temp_file, err := os.CreateTemp("", "tubely-upload.mp4")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "unable to create temp file", nil)
		return

	}
	defer os.Remove(temp_file.Name())

	defer temp_file.Close()

	_, err = io.Copy(temp_file, file)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to save file on disk", err)
		return
	}

	temp_file.Seek(0, io.SeekStart)

	processed_path, err := processVideoForFastStart(temp_file.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to process video for fast start", err)
		return
	}

	processed_temp_file, err := os.Open(processed_path)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "unable to open processed file", err)
		return
	}

	defer os.Remove(processed_temp_file.Name())
	defer processed_temp_file.Close()

	aspect_ratio, err := getVideoAspectRatio(temp_file.Name())
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Could not generate aspect ratio", err)
	}

	processed_temp_file.Seek(0, io.SeekStart)

	random_bytes := make([]byte, 32)
	rand.Read(random_bytes)
	random_name := base64.RawURLEncoding.EncodeToString(random_bytes)
	extension := strings.Split(media_type, "/")[1]
	video_name := fmt.Sprintf("%s%s.%s", aspect_ratio, random_name, extension)

	_, err = cfg.s3Client.PutObject(r.Context(), &s3.PutObjectInput{
		Bucket:      aws.String(cfg.s3Bucket),
		Key:         aws.String(video_name),
		Body:        processed_temp_file,
		ContentType: aws.String(media_type),
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "Unable to upload object to s3", err)
		return
	}

	video_url := fmt.Sprintf("%s/%s", cfg.s3CfDistribution, video_name)
	video.VideoURL = &video_url
	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, http.StatusNoContent, "Unable to update video", err)
		return
	}
}
