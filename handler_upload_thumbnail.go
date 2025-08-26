package main

import (
	"database/sql"
	"encoding/base64"
	"fmt"
	"io"
	"net/http"
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

	dat, err := io.ReadAll(file)
	if err != nil {
		respondWithError(w, 500, "Unable to read file", err)
		return
	}

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

	split := strings.Split(header.Filename, ".")
	t := strings.Join(split[1:], ".")
	full := fmt.Sprintf("image/%s", t)
	result := thumbnail{
		data:      dat,
		mediaType: full,
	}

	data := base64.StdEncoding.EncodeToString(result.data)

	tb := fmt.Sprintf("data:%s;base64,%s", result.mediaType, data)
	video.ThumbnailURL = &tb
	err = cfg.db.UpdateVideo(video)
	if err != nil {
		respondWithError(w, 500, "Unable to update video with thumbnail", err)
		return
	}

	video, _ = cfg.db.GetVideo(videoID)

	respondWithJSON(w, http.StatusOK, video)
}
