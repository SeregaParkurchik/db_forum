package config

import (
	"errors"
	"net"
	"os"
)

func NewServerAddress() (string, error) {
	host := os.Getenv("HTTP_HOST")
	if len(host) == 0 {
		return "", errors.New("host не найден в .env файле")
	}

	port := os.Getenv("HTTP_PORT")
	if len(port) == 0 {
		return "", errors.New("port не найден в .env файле")
	}

	addres := net.JoinHostPort(host, port)

	return addres, nil
}
