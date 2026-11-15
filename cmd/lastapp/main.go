package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"github.com/aantich/lastapp/internal/control"
)

func main() {
	if len(os.Args) < 2 {
		log.Fatal("usage: lastapp serve|migrate|bootstrap|seed-development -config path")
	}
	command := os.Args[1]
	flags := flag.NewFlagSet(command, flag.ExitOnError)
	config := flags.String("config", ".local/config.json", "server configuration file")
	issuer := flags.String("issuer", "", "bootstrap identity issuer")
	subject := flags.String("subject", "", "bootstrap immutable identity subject")
	name := flags.String("name", "Platform operator", "bootstrap name")
	email := flags.String("email", "", "bootstrap email")
	provisionerFile := flags.String("provisioner-file", "", "local development provisioning credential file")
	passwordFile := flags.String("password-file", "", "initial password file for a local bootstrap account")
	flags.Parse(os.Args[2:])
	c, e := control.LoadConfig(*config)
	if e != nil {
		log.Fatal(e)
	}
	s, e := control.Open(c)
	if e != nil {
		log.Fatal(e)
	}
	defer s.Close()
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	switch command {
	case "migrate":
		e = control.Migrate(ctx, s.DB)
	case "bootstrap":
		if *passwordFile != "" || (*issuer == "" && *subject == "" && *email != "") {
			if *issuer != "" || *subject != "" {
				log.Fatal("choose local account or OIDC bootstrap, not both")
			}
			var secret []byte
			if *passwordFile != "" {
				secret, e = os.ReadFile(*passwordFile)
				if e != nil {
					break
				}
			}
			var uid string
			uid, e = s.BootstrapLocal(ctx, *name, *email, strings.TrimRight(string(secret), "\r\n"))
			if e == nil {
				fmt.Println("Superadmin identity:", uid)
			}
			break
		}
		if *issuer == "" || *subject == "" {
			log.Fatal("issuer and subject are required")
		}
		var uid string
		uid, e = s.Bootstrap(ctx, *issuer, *subject, *name, *email)
		if e == nil {
			fmt.Println("Superadmin identity:", uid)
		}
	case "seed-development":
		var secret []byte
		if *provisionerFile != "" {
			secret, e = os.ReadFile(*provisionerFile)
			if e != nil {
				break
			}
		}
		e = s.SeedDevelopment(ctx, strings.TrimSpace(string(secret)))
	case "serve":
		if e = control.CheckSchema(ctx, s.DB); e != nil {
			break
		}
		if e = s.InitAuth(ctx); e != nil {
			break
		}
		go s.RunWorker(ctx)
		server := &http.Server{Addr: c.Address, Handler: s.Handler(), ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 30 * time.Second, WriteTimeout: 60 * time.Second, IdleTimeout: 60 * time.Second, MaxHeaderBytes: 32 << 10}
		stopped := make(chan struct{})
		go func() {
			defer close(stopped)
			<-ctx.Done()
			shutdown, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()
			server.Shutdown(shutdown)
		}()
		log.Printf("Lastapp listening at %s/console/", c.PublicURL)
		e = server.ListenAndServe()
		if e == http.ErrServerClosed {
			<-stopped
			e = nil
		}
	default:
		e = fmt.Errorf("unknown command %q", command)
	}
	if e != nil {
		log.Fatal(e)
	}
}
