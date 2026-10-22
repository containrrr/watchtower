package types

// SystemInfo is a subset of Docker/Podman daemon info used by Watchtower.
type SystemInfo struct {
	Name            string
	ServerVersion   string
	OSType          string
	OperatingSystem string
	Driver          string
}
