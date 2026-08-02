package server

import (
	"fmt"
	"math"
	"net/netip"
	"strings"
)

const (
	megabyte            int64 = 1024 * 1024
	requestBodyOverhead       = megabyte
)

// ValidateOptions validates server settings before model loading or binding.
func ValidateOptions(options *ServerOptions) error {
	if options == nil {
		return fmt.Errorf("server options are required")
	}
	if options.Host != "localhost" {
		if _, err := netip.ParseAddr(options.Host); err != nil {
			return fmt.Errorf("invalid host %q: use an IP address or localhost", options.Host)
		}
	}
	if options.Port < 1 || options.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535")
	}
	if options.Workers <= 0 {
		return fmt.Errorf("workers must be positive")
	}
	if options.MaxQueueSize <= 0 {
		return fmt.Errorf("queue size must be positive")
	}
	if options.MaxQueueSize > int(^uint(0)>>1)-options.Workers {
		return fmt.Errorf("workers and queue size are too large")
	}
	if options.MaxUploadMB <= 0 {
		return fmt.Errorf("maximum upload must be positive")
	}
	maxUploadMB := ((math.MaxInt64-requestBodyOverhead)/4*3 - 2) / megabyte
	if options.MaxUploadMB > maxUploadMB {
		return fmt.Errorf("maximum upload is too large")
	}
	return nil
}

func (o *ServerOptions) maxUploadBytes() int64 {
	return o.MaxUploadMB * megabyte
}

func (o *ServerOptions) maxRequestBodyBytes(contentType string) int64 {
	maxUpload := o.maxUploadBytes()
	if strings.HasPrefix(contentType, "application/json") {
		return ((maxUpload + 2) / 3 * 4) + requestBodyOverhead
	}
	return maxUpload + requestBodyOverhead
}
