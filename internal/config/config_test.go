package config

import (
	"flag"
	"testing"

	"github.com/magiconair/properties/assert"
)

type FakeEnv map[string]string

func (f FakeEnv) LookupEnv(key string) (string, bool) {
	value, exists := f[key]
	return value, exists
}

func TestLoad(t *testing.T) {
	tests := []struct {
		name    string
		fs      *flag.FlagSet
		args    []string
		env     EnvSource
		want    *AppConfig
		wantErr bool
	}{
		{
			name: "empty flags and empty env",
			fs:   flag.NewFlagSet("server_test", flag.ContinueOnError),
			args: []string{},
			env:  FakeEnv{},
			want: &AppConfig{
				AccrualSystemAddress: "",
				DatabaseURI:          "",
				RunAddress:           "localhost:8080",
			},
			wantErr: true,
		},
		{
			name: "flags values",
			fs:   flag.NewFlagSet("server_test", flag.ContinueOnError),
			args: []string{"-a", "192.168.1.1:8080", "-d", "postgresql://test_user:qwerty123@localhost:5433/tests?sslmode=disable", "-r", "192.168.1.2:8080"},
			env:  FakeEnv{},
			want: &AppConfig{
				AccrualSystemAddress: "192.168.1.2:8080",
				DatabaseURI:          "postgresql://test_user:qwerty123@localhost:5433/tests?sslmode=disable",
				RunAddress:           "192.168.1.1:8080",
			},
		},
		{
			name: "env over flags values",
			fs:   flag.NewFlagSet("server_test", flag.ContinueOnError),
			args: []string{"-a", "192.168.1.1:8080", "-d", "postgresql://test_user:qwerty123@localhost:5433/tests?sslmode=disable", "-r", "192.168.1.2:8080"},
			env: FakeEnv{
				accrualSystemAddress: "192.168.1.4:8081",
				databaseURI:          "postgresql://test_user_env:qwerty123@localhost:5433/tests?sslmode=disable",
				runAddress:           "192.168.1.3:8080",
			},
			want: &AppConfig{
				AccrualSystemAddress: "192.168.1.4:8081",
				DatabaseURI:          "postgresql://test_user_env:qwerty123@localhost:5433/tests?sslmode=disable",
				RunAddress:           "192.168.1.3:8080",
			},
		},
		{
			name: "env over default values",
			fs:   flag.NewFlagSet("server_test", flag.ContinueOnError),
			args: []string{},
			env: FakeEnv{
				accrualSystemAddress: "192.168.1.4:8081",
				databaseURI:          "postgresql://test_user_env:qwerty123@localhost:5433/tests?sslmode=disable",
				runAddress:           "192.168.1.3:8080",
			},
			want: &AppConfig{
				AccrualSystemAddress: "192.168.1.4:8081",
				DatabaseURI:          "postgresql://test_user_env:qwerty123@localhost:5433/tests?sslmode=disable",
				RunAddress:           "192.168.1.3:8080",
			},
		},
		{
			name: "invalidFlag",
			fs:   flag.NewFlagSet("server_test", flag.ContinueOnError),
			args: []string{"-a", "192.168.1.1:8080", "-invalidFlag", "true"},
			env: FakeEnv{
				accrualSystemAddress: "192.168.1.4:8081",
				databaseURI:          "postgresql://test_user_env:qwerty123@localhost:5433/tests?sslmode=disable",
				runAddress:           "192.168.1.3:8080",
			},
			want: &AppConfig{
				AccrualSystemAddress: "192.168.1.4:8081",
				DatabaseURI:          "postgresql://test_user_env:qwerty123@localhost:5433/tests?sslmode=disable",
				RunAddress:           "192.168.1.3:8080",
			},
			wantErr: true,
		},
		{
			name: "empty env values #1",
			fs:   flag.NewFlagSet("server_test", flag.ContinueOnError),
			args: []string{"-a", "192.168.1.1:8080", "-d", "postgresql://test_user:qwerty123@localhost:5433/tests?sslmode=disable", "-r", "192.168.1.2:8080"},
			env: FakeEnv{
				accrualSystemAddress: "",
				databaseURI:          "",
				runAddress:           "",
			},
			want: &AppConfig{
				AccrualSystemAddress: "192.168.1.2:8080",
				DatabaseURI:          "postgresql://test_user:qwerty123@localhost:5433/tests?sslmode=disable",
				RunAddress:           "192.168.1.1:8080",
			},
			wantErr: false,
		},
		{
			name: "empty env values #2",
			fs:   flag.NewFlagSet("server_test", flag.ContinueOnError),
			args: []string{},
			env: FakeEnv{
				accrualSystemAddress: "",
				databaseURI:          "",
				runAddress:           "",
			},
			want: &AppConfig{
				AccrualSystemAddress: "",
				DatabaseURI:          "",
				RunAddress:           ":8080",
			},
			wantErr: true,
		},
		{
			name: "mixied env and flags and default values",
			fs:   flag.NewFlagSet("server_test", flag.ContinueOnError),
			args: []string{"-d", "postgresql://test_user:qwerty123@localhost:5433/tests?sslmode=disable"},
			env: FakeEnv{
				accrualSystemAddress: "192.168.1.4:8081",
			},
			want: &AppConfig{
				AccrualSystemAddress: "192.168.1.4:8081",
				DatabaseURI:          "postgresql://test_user:qwerty123@localhost:5433/tests?sslmode=disable",
				RunAddress:           ":8080",
			},
		},
		{
			name: "empty RunAddress param",
			fs:   flag.NewFlagSet("server_test", flag.ContinueOnError),
			args: []string{"-a"},
			env: FakeEnv{
				accrualSystemAddress: "192.168.1.4:8081",
				databaseURI:          "postgresql://test_user:qwerty123@localhost:5433/tests?sslmode=disable",
			},
			want: &AppConfig{
				AccrualSystemAddress: "192.168.1.4:8081",
				DatabaseURI:          "postgresql://test_user:qwerty123@localhost:5433/tests?sslmode=disable",
				RunAddress:           ":8080",
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, gotErr := Load(tt.fs, tt.args, tt.env)

			if gotErr != nil {
				if !tt.wantErr {
					t.Errorf("Parse() failed: %v", gotErr)
				}
				return
			}
			if tt.wantErr {
				t.Fatal("Parse() succeeded unexpectedly")
			}

			assert.Equal(t, tt.want, got)
		})
	}
}

func TestGetEnvOrArg(t *testing.T) {
	tests := []struct {
		name     string
		envVars  map[string]string
		envKey   string
		cliArg   string
		expected string
	}{
		{
			name:     "env is set and not empty -> return env",
			envVars:  map[string]string{databaseURI: "postgres://prod"},
			envKey:   databaseURI,
			cliArg:   "postgres://local",
			expected: "postgres://prod",
		},
		{
			name:     "env is empty string -> fallback to cli",
			envVars:  map[string]string{databaseURI: ""},
			envKey:   databaseURI,
			cliArg:   "postgres://local",
			expected: "postgres://local",
		},
		{
			name:     "env is not set -> fallback to cli",
			envVars:  map[string]string{},
			envKey:   databaseURI,
			cliArg:   "postgres://local",
			expected: "postgres://local",
		},
		{
			name:     "both empty -> return empty",
			envVars:  map[string]string{},
			envKey:   databaseURI,
			cliArg:   "",
			expected: "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			got := getEnvOrArg(FakeEnv(tt.envVars), tt.envKey, tt.cliArg)
			if got != tt.expected {
				t.Errorf("getEnvOrArg() = %q, want %q", got, tt.expected)
			}
		})
	}
}
