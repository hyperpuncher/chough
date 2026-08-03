package server

import "testing"

func TestValidateOptions(t *testing.T) {
	valid := DefaultOptions()
	if err := ValidateOptions(valid); err != nil {
		t.Fatalf("ValidateOptions() error = %v", err)
	}

	tests := []struct {
		name   string
		mutate func(*ServerOptions)
	}{
		{name: "invalid host", mutate: func(o *ServerOptions) { o.Host = "not a host" }},
		{name: "zero port", mutate: func(o *ServerOptions) { o.Port = 0 }},
		{name: "large port", mutate: func(o *ServerOptions) { o.Port = 65536 }},
		{name: "zero workers", mutate: func(o *ServerOptions) { o.Workers = 0 }},
		{name: "zero queue", mutate: func(o *ServerOptions) { o.MaxQueueSize = 0 }},
		{name: "zero upload", mutate: func(o *ServerOptions) { o.MaxUploadMB = 0 }},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			options := valid
			tt.mutate(&options)
			if err := ValidateOptions(options); err == nil {
				t.Fatal("ValidateOptions() error = nil, want validation error")
			}
		})
	}
}

func TestRequestBodyLimits(t *testing.T) {
	options := ServerOptions{MaxUploadMB: 1}
	if got, want := options.maxRequestBodyBytes("multipart/form-data"), int64(2*megabyte); got != want {
		t.Fatalf("multipart body limit = %d, want %d", got, want)
	}
	if got, want := options.maxRequestBodyBytes("application/json"), int64(2446680); got != want {
		t.Fatalf("JSON body limit = %d, want %d", got, want)
	}
}
