package main

import (
	"context"
	"crypto/tls"
	"crypto/x509"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	locksmith "github.com/maansthoernvik/locksmith/pkg"
	"github.com/maansthoernvik/locksmith/pkg/env"
	"github.com/maansthoernvik/locksmith/pkg/log"
	"github.com/maansthoernvik/locksmith/pkg/vault"
	"github.com/maansthoernvik/locksmith/pkg/version"
)

func main() {
	// Set global log level
	logLevel, _ := env.GetOptionalString(env.LOCKSMITH_LOG_LEVEL, env.LOCKSMITH_LOG_LEVEL_DEFAULT)
	log.SetLogLevel(log.Translate(logLevel))

	// Print to bypass loglevel settings and write to stdout
	// Check if '?' since the version info can only be set for container builds, not via 'go install'
	if version.Version != "?" {
		fmt.Printf(
			"starting Locksmith... \nversion: %s\ncommit: %s\nbuilt: %s\n",
			version.Version, version.Commit, version.Built,
		)
	} else {
		fmt.Println("starting Locksmith...")
	}

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		signal_ch := make(chan os.Signal, 1)
		signal.Notify(signal_ch, syscall.SIGINT, syscall.SIGTERM)
		signal := <-signal_ch
		log.Info("got signal: ", signal)
		cancel()
	}()

	port, _ := env.GetOptionalUint16(env.LOCKSMITH_PORT, env.LOCKSMITH_PORT_DEFAULT)
	queueType, _ := env.GetOptionalString(env.LOCKSMITH_Q_TYPE, env.LOCKSMITH_Q_TYPE_DEFAULT)
	concurrency, _ := env.GetOptionalInteger(env.LOCKSMITH_Q_CONCURRENCY, env.LOCKSMITH_Q_CONCURRENCY_DEFAULT)
	capacity, _ := env.GetOptionalInteger(env.LOCKSMITH_Q_CAPACITY, env.LOCKSMITH_Q_CAPACITY_DEFAULT)

	locksmithOptions := &locksmith.LocksmithOptions{
		Port:             port,
		QueueType:        vault.QueueType(queueType),
		QueueConcurrency: concurrency,
		QueueCapacity:    capacity,
	}
	if tls, _ := env.GetOptionalBool(env.LOCKSMITH_TLS, env.LOCKSMITH_TLS_DEFAULT); tls {
		locksmithOptions.TlsConfig = getTlsConfig()
	}
	if err := locksmith.New(locksmithOptions).Start(ctx); err != nil {
		log.Error("server start error: ", err)
		os.Exit(1)
	}

	log.Info("server stopped")
}

// Fetch TLS config to supply the TCP listener.
func getTlsConfig() *tls.Config {
	tlsConfig := &tls.Config{}

	serverCertPath, _ := env.GetOptionalString(env.LOCKSMITH_TLS_CERT_PATH, env.LOCKSMITH_TLS_CERT_PATH_DEFAULT)
	serverKeyPath, _ := env.GetOptionalString(env.LOCKSMITH_TLS_KEY_PATH, env.LOCKSMITH_TLS_KEY_PATH_DEFAULT)
	cert, err := tls.LoadX509KeyPair(serverCertPath, serverKeyPath)
	if err != nil {
		panic("failed to load server cert/key pair")
	}
	tlsConfig.Certificates = []tls.Certificate{cert}

	requireClientVerify, _ := env.GetOptionalBool(env.LOCKSMITH_TLS_REQUIRE_CLIENT_CERT, env.LOCKSMITH_TLS_REQUIRE_CLIENT_CERT_DEFAULT)
	if requireClientVerify {
		clientCaCertPath, _ := env.GetOptionalString(env.LOCKSMITH_TLS_CLIENT_CA_CERT_PATH, env.LOCKSMITH_TLS_CLIENT_CA_CERT_PATH_DEFAULT)
		tlsConfig.ClientAuth = tls.RequireAndVerifyClientCert
		caCert, err := os.ReadFile(clientCaCertPath)
		if err != nil {
			panic("failed to read client CA cert file")
		}
		pool := x509.NewCertPool()
		pool.AppendCertsFromPEM(caCert)
		tlsConfig.ClientCAs = pool
	}

	return tlsConfig
}
