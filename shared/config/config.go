package config

import "time"

type Database struct {
	Host     string
	Port     int
	Username string
	Password string
	Database string
	SSLMode  string
}

type Redis struct {
	Addr     string
	Password string
	DB       int
}

type MQTT struct {
	Broker   string
	Username string
	Password string
	ClientID string
}

type Minio struct {
	Endpoint  string
	AccessKey string
	SecretKey string
	Bucket    string
}

type OpenSearch struct {
	Endpoints []string
	Username  string
	Password  string
}

type Server struct {
	HTTP struct {
		Network string
		Addr    string
		Timeout time.Duration
	}
	GRPC struct {
		Network string
		Addr    string
		Timeout time.Duration
	}
}
