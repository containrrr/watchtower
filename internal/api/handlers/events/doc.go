// Package events provides the /v1/events endpoint, which streams Watchtower's
// scan and image cleanup events to clients as Server-Sent Events, and the
// broadcaster that delivers each event to every subscriber.
package events
