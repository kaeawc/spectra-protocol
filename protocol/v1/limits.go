package v1

// Wire and envelope limits.
const (
	MaxRequestBytes      = 1 << 20
	MaxResponseBytes     = 16 << 20
	MaxRequestIDLen      = 128
	MaxTimeoutMS         = 600000
	MaxErrorMessageBytes = 4096
)
