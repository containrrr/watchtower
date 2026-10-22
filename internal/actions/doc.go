// Package actions runs Watchtower's update sessions.
//
// A session lists the monitored containers, finds the ones whose image or Git
// source has changed, and recreates them and the containers that depend on
// them in dependency order. Watchtower updates its own container last, and
// removes replaced images when cleanup is enabled.
package actions
