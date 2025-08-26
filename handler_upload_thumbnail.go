package main

import (
	"database/sql"
	"fmt"
	"io"
	"mime"
	"net/http"
	"os"
	"strings"

	"github.com/bootdotdev/learn-file-storage-s3-golang-starter/internal/auth"
	"github.com/google/uuid"
)

func (cfg *apiConfig) handlerUploadThumbnail(w http.ResponseWriter, r *http.Request) {
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

	fmt.Println("uploading thumbnail for video", videoID, "by user", userID)

	// TODO: implement the upload here

	const maxMemory = 10 << 20

	err = r.ParseMultipartForm(maxMemory)
	if err != nil {
		respondWithError(w, 500, "Unable to parse Multipart Form", err)
		return
	}

	file, header, err := r.FormFile("thumbnail")
	if err != nil {
		respondWithError(w, http.StatusBadRequest, "Unable to parse form file", err)
		return
	}
	defer file.Close()

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

	name, _, err := mime.ParseMediaType(header.Filename)
	split := strings.Split(name, ".")
	ext := strings.Join(split[1:], ".")
	if err != nil {
		respondWithError(w, 500, "Unable to parse media type", err)
		return
	} else if ext != "jpeg" && ext != "png" {
		respondWithError(w, 422, "Wrong file format for thumbnail", nil)
		return
	}

	fileName := fmt.Sprintf("%s/%s.%s", cfg.assetsRoot, videoID.String(), ext)
	f, err := os.Create(fileName)
	if err != nil {
		respondWithError(w, 500, "Unable to create file for thumbnail", err)
		return
	}
	defer f.Close()

	_, err = io.Copy(f, file)
	if err != nil {
		respondWithError(w, 500, "Unable to copy thumbnail data to new file", err)
		return
	}

	tb := fmt.Sprintf("http://localhost:8091%s", fileName[1:])
	video.ThumbnailURL = &tb
	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, 500, "Unable to update video with thumbnail", err)
		return
	}

	video, _ = cfg.db.GetVideo(videoID)

	respondWithJSON(w, http.StatusOK, video)
}
