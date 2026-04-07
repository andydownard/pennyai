// Groflo — main.go (formerly Thriftly)
//
// This file is intentionally minimal. All logic has been split into:
//   server.go          — config, main(), route registration
//   db.go              — database initialization and helpers
//   models.go          — struct types
//   session.go         — SessionStore and Twilio client
//   utils.go           — HTTP helpers (writeJSON, requireMethod, etc.)
//   penny.go           — AI/Claude integration
//   handlers_auth.go   — phone, email, Google auth
//   handlers_list.go   — grocery list CRUD + brand logo proxy
//   handlers_settings.go — settings, trips, NPS, referrals
//   handlers_social.go — invite, join, household
//   handlers_admin.go  — admin dashboard, legal pages
//   handlers_saved_lists.go — saved list CRUD
//   handlers_webhook.go — SMS and email webhooks

package main
