package main

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/s3"
	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadVideo(w http.ResponseWriter, r *http.Request) {
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

	fmt.Println("uploading video", videoID, "by user", userID)

	const maxMemory = 1 << 30

	http.MaxBytesReader(w, r.Body, maxMemory)

	video, err := cfg.db.GetVideo(videoID)
	if err == sql.ErrNoRows {
		respondWithError(w, 404, "Video does not exist", err)
		return
	} else if err != nil {
		respondWithError(w, 500, "Unable to retrieve video", err)
		return
	} else if video.UserID != userID {
		respondWithError(w, http.StatusUnauthorized, "Authenticated user is not the video owner", err)
		return
	}

	file, header, err := r.FormFile("video")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}
	defer file.Close()

	name, _, err := mime.ParseMediaType(header.Filename)
	split := strings.Split(name, ".")
	ext := strings.Join(split[1:], ".")
	if err != nil {
		respondWithError(w, 500, "Unable to parse media type", err)
		return
	} else if ext != "mp4" {
		respondWithError(w, 422, "Wrong file format for video", nil)
		return
	}
	mime_type := fmt.Sprintf("video/%s", ext)

	temp, err := os.CreateTemp("", "tubely-upload.mp4")
	if err != nil {
		respondWithError(w, 500, "Unable to create temp file", err)
		return
	}
	defer os.Remove("tubely-upload.mp4")
	defer temp.Close()

	_, err = io.Copy(temp, file)
	if err != nil {
		respondWithError(w, 500, "Unable to copy contents to temp file", err)
		return
	}

	_, err = temp.Seek(0, io.SeekStart)
	if err != nil {
		respondWithError(w, 500, "Unable to reset file pointer", err)
		return
	}

	b := make([]byte, 32)
	_, err = rand.Read(b)
	if err != nil {
		respondWithError(w, 500, "Unable to create video key", err)
		return
	}

	key := base64.RawURLEncoding.EncodeToString(b)

	params := &s3.PutObjectInput{
		Bucket:      &cfg.s3Bucket,
		Key:         &key,
		Body:        temp,
		ContentType: &mime_type,
	}
	_, err = cfg.s3Client.PutObject(r.Context(), params)
	if err != nil {
		respondWithError(w, 500, "Unable to upload video to bucket", err)
		return
	}

	url := fmt.Sprintf("https://%s.s3.%s.amazonaws.com/%s", cfg.s3Bucket, cfg.s3Region, key)
	video.VideoURL = &url
	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, 500, "Unable to update video URL", err)
		return
	}

	video, _ = cfg.db.GetVideo(videoID)

	respondWithJSON(w, http.StatusOK, video)
}
