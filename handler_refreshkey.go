package main

import (
	"log"
	"net/http"
	"time"

	"github.com/STaninnat/capstone_project/internal/database"
	"github.com/google/uuid"
)

func (apicfg *apiConfig) handlerRefreshKey(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie("refresh_token")
	if err != nil {
		respondWithError(w, http.StatusUnauthorized, "couldn't find refresh token")
		return
	}
	refreshToken := cookie.Value

	user, err := apicfg.DB.GetUserByRfKey(r.Context(), refreshToken)
	if err != nil || user.RefreshTokenExpiresAt.Before(time.Now().UTC()) {
		respondWithError(w, http.StatusUnauthorized, "invalid or expired refresh token")
		return
	}

	_, newHashedApiKey, err := generateAndHashAPIKey()
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "couldn't generate new apikey")
		return
	}

	newApiKeyExpiresAt := time.Now().UTC().Add(365 * 24 * time.Hour)
	newAccessTokenExpiresAt := time.Now().UTC().Add(1 * time.Hour)

	userID, err := uuid.Parse(user.UserID)
	if err != nil {
		log.Printf("Error parsing user ID: %v", err)
		respondWithError(w, http.StatusInternalServerError, "invalid user ID")
		return
	}

	newAccessToken, err := generateJWTToken(userID, apicfg.JWTSecret, newAccessTokenExpiresAt)
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "couldn't generate new access token")
		return
	}

	err = apicfg.DB.UpdateUser(r.Context(), database.UpdateUserParams{
		UpdatedAt:       time.Now().UTC(),
		ApiKey:          newHashedApiKey,
		ApiKeyExpiresAt: newApiKeyExpiresAt,
		ID:              user.UserID,
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "failed to update apikey")
		return
	}

	newRefreshTokenExpiresAt := time.Now().UTC().Add(30 * 24 * time.Hour)
	err = apicfg.DB.UpdateUserRfKey(r.Context(), database.UpdateUserRfKeyParams{
		UpdatedAt:             time.Now().UTC(),
		AccessTokenExpiresAt:  newAccessTokenExpiresAt,
		RefreshToken:          refreshToken,
		RefreshTokenExpiresAt: newRefreshTokenExpiresAt,
		UserID:                user.UserID,
	})
	if err != nil {
		respondWithError(w, http.StatusInternalServerError, "failed to update refresh token")
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     "access_token",
		Value:    newAccessToken,
		Expires:  newAccessTokenExpiresAt,
		HttpOnly: true,
		Path:     "/",
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		// SameSite: http.SameSiteLaxMode,
	})

	http.SetCookie(w, &http.Cookie{
		Name:     "refresh_token",
		Value:    refreshToken,
		Expires:  newRefreshTokenExpiresAt,
		HttpOnly: true,
		Path:     "/",
		Secure:   true,
		SameSite: http.SameSiteStrictMode,
		// SameSite: http.SameSiteLaxMode,
	})

	userResp := map[string]interface{}{
		"message": "token refreshed successfully",
	}

	respondWithJSON(w, http.StatusOK, userResp)
}
