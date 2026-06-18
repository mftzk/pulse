package httpapi

import (
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"
)

func (s *Server) handleListOrgs(w http.ResponseWriter, r *http.Request) {
	user := userFrom(r.Context())
	orgs, err := s.store.ListOrgsForUser(r.Context(), user.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, orgs)
}

func (s *Server) handleCreateOrg(w http.ResponseWriter, r *http.Request) {
	user := userFrom(r.Context())
	var in struct {
		Name string `json:"name"`
	}
	if err := readJSON(r, &in); err != nil || strings.TrimSpace(in.Name) == "" {
		writeErr(w, http.StatusBadRequest, "name is required")
		return
	}
	org, err := s.createOrgUnique(r.Context(), strings.TrimSpace(in.Name), user.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not create org")
		return
	}
	writeJSON(w, http.StatusCreated, org)
}

func (s *Server) handleGetOrg(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, orgFrom(r.Context()))
}

func (s *Server) handleListMembers(w http.ResponseWriter, r *http.Request) {
	org := orgFrom(r.Context())
	members, err := s.store.ListMembers(r.Context(), org.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, members)
}

func (s *Server) handleAddMember(w http.ResponseWriter, r *http.Request) {
	org := orgFrom(r.Context())
	if org.Role != "owner" {
		writeErr(w, http.StatusForbidden, "only owners can add members")
		return
	}
	var in struct {
		Username string `json:"username"`
		Role     string `json:"role"`
	}
	if err := readJSON(r, &in); err != nil || strings.TrimSpace(in.Username) == "" {
		writeErr(w, http.StatusBadRequest, "username is required")
		return
	}
	if in.Role != "owner" {
		in.Role = "member"
	}
	member, err := s.store.AddMemberByUsername(r.Context(), org.ID, strings.TrimSpace(in.Username), in.Role)
	if err != nil {
		notFoundOr500(w, err) // ErrNotFound => no such user
		return
	}
	writeJSON(w, http.StatusCreated, member)
}

func (s *Server) handleRemoveMember(w http.ResponseWriter, r *http.Request) {
	org := orgFrom(r.Context())
	user := userFrom(r.Context())
	target := chi.URLParam(r, "userID")
	if org.Role != "owner" && target != user.ID {
		writeErr(w, http.StatusForbidden, "only owners can remove other members")
		return
	}
	if err := s.store.RemoveMember(r.Context(), org.ID, target); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleListChannels(w http.ResponseWriter, r *http.Request) {
	org := orgFrom(r.Context())
	chans, err := s.store.ListChannels(r.Context(), org.ID)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	writeJSON(w, http.StatusOK, chans)
}

func (s *Server) handleCreateChannel(w http.ResponseWriter, r *http.Request) {
	org := orgFrom(r.Context())
	var in struct {
		Type       string `json:"type"`
		Name       string `json:"name"`
		WebhookURL string `json:"webhook_url"`
	}
	if err := readJSON(r, &in); err != nil || !strings.HasPrefix(in.WebhookURL, "http") {
		writeErr(w, http.StatusBadRequest, "a valid webhook_url is required")
		return
	}
	in.Type = strings.TrimSpace(strings.ToLower(in.Type))
	if in.Type == "" {
		in.Type = "discord"
	}
	if in.Type != "discord" && in.Type != "slack" {
		writeErr(w, http.StatusBadRequest, "type must be 'discord' or 'slack'")
		return
	}
	if strings.TrimSpace(in.Name) == "" {
		if in.Type == "slack" {
			in.Name = "Slack"
		} else {
			in.Name = "Discord"
		}
	}
	ch, err := s.store.CreateChannel(r.Context(), org.ID, in.Type, in.Name, in.WebhookURL)
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "could not create channel")
		return
	}
	writeJSON(w, http.StatusCreated, ch)
}

func (s *Server) handleDeleteChannel(w http.ResponseWriter, r *http.Request) {
	org := orgFrom(r.Context())
	if err := s.store.DeleteChannel(r.Context(), org.ID, chi.URLParam(r, "id")); err != nil {
		writeErr(w, http.StatusInternalServerError, "internal error")
		return
	}
	w.WriteHeader(http.StatusNoContent)
}
