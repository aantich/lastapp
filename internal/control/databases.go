package control

import (
	"context"
	"crypto/tls"
	"database/sql"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	mariadb "github.com/aantich/go-mariadb"

	"github.com/aantich/lastapp/internal/access"
	"github.com/aantich/lastapp/internal/databases"
)

func (s *Server) targets(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	v, e := s.storage.Targets(r.Context(), httpActor(r, p), pageRequest(r.URL.Query()))
	if e == nil {
		send(w, 200, v)
	}
	return e
}

func (s *Server) availableTargets(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	v, e := s.storage.AvailableTargets(r.Context(), httpActor(r, p), r.PathValue("tenant"), r.URL.Query().Get("managed") == "true", pageRequest(r.URL.Query()))
	if e == nil {
		send(w, 200, v)
	}
	return e
}

func (s *Server) createTarget(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	var in databases.TargetInput
	if e := decode(w, r, &in); e != nil {
		return e
	}
	v, e := s.storage.CreateTarget(r.Context(), httpActor(r, p), in)
	if e == nil {
		sendRef(w, 201, v)
	}
	return e
}

func (s *Server) updateTarget(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	var in databases.TargetPatch
	if e := decode(w, r, &in); e != nil {
		return e
	}
	v, e := s.storage.UpdateTarget(r.Context(), httpActor(r, p), r.PathValue("id"), in, ifMatch(r))
	if e == nil {
		sendRef(w, 200, v)
	}
	return e
}

func (s *Server) databases(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	v, e := s.storage.Databases(r.Context(), httpActor(r, p), r.PathValue("tenant"), pageRequest(r.URL.Query()))
	if e == nil {
		send(w, 200, v)
	}
	return e
}

func (s *Server) getDatabase(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	v, e := s.storage.Database(r.Context(), httpActor(r, p), r.PathValue("tenant"), r.PathValue("id"))
	if e == nil {
		send(w, 200, v)
	}
	return e
}

func (s *Server) createDatabase(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	var in databases.DatabaseInput
	if e := decode(w, r, &in); e != nil {
		return e
	}
	v, e := s.storage.CreateDatabase(r.Context(), httpActor(r, p), r.PathValue("tenant"), in)
	if e == nil {
		sendRef(w, 202, v)
	}
	return e
}

func (s *Server) retryDatabase(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	v, e := s.storage.RetryDatabase(r.Context(), httpActor(r, p), r.PathValue("tenant"), r.PathValue("id"), ifMatch(r))
	if e == nil {
		sendRef(w, 202, v)
	}
	return e
}

func (s *Server) updateDatabase(w http.ResponseWriter, r *http.Request, p access.Principal) error {
	var in databases.DatabaseChange
	if e := decode(w, r, &in); e != nil {
		return e
	}
	v, e := s.storage.UpdateDatabase(r.Context(), httpActor(r, p), r.PathValue("tenant"), r.PathValue("id"), in, ifMatch(r))
	if e == nil {
		sendRef(w, 202, v)
	}
	return e
}

func (s *Server) targetDB(ctx context.Context, target databases.DatabaseEndpoint, cred databases.Credentials, name string) (*sql.DB, error) {
	host := target.Host
	port := strconv.Itoa(target.Port)
	cfg := mariadb.NewConfig()
	cfg.User = cred.Username
	cfg.Passwd = cred.Password
	cfg.Net = "tcp"
	cfg.Addr = net.JoinHostPort(host, port)
	cfg.DBName = name
	cfg.ParseTime = true
	cfg.Timeout = 5 * time.Second
	cfg.ReadTimeout = 15 * time.Second
	cfg.WriteTimeout = 15 * time.Second
	if target.TLS == "verify_identity" {
		cfg.TLS = &tls.Config{ServerName: host, MinVersion: tls.VersionTLS12}
	} else if !s.Config.Development {
		return nil, fmt.Errorf("TLS is required")
	}
	cfg.DialFunc = func(c context.Context, network, address string) (net.Conn, error) {
		ips, e := net.DefaultResolver.LookupIPAddr(c, host)
		if e != nil {
			return nil, e
		}
		for _, ip := range ips {
			allowed := !ip.IP.IsPrivate() && !ip.IP.IsLoopback() && !ip.IP.IsUnspecified() && !ip.IP.IsLinkLocalUnicast() && !ip.IP.IsMulticast()
			if s.Config.Development {
				allowed = ip.IP.IsLoopback()
			}
			for _, cidr := range s.Config.AllowedNetworks {
				_, sub, _ := net.ParseCIDR(cidr)
				if sub != nil && sub.Contains(ip.IP) {
					allowed = true
				}
			}
			if !allowed {
				return nil, fmt.Errorf("database address is not allowed")
			}
		}
		if len(ips) == 0 {
			return nil, fmt.Errorf("no database address")
		}
		return (&net.Dialer{Timeout: 5 * time.Second}).DialContext(c, network, net.JoinHostPort(ips[0].IP.String(), port))
	}
	connector, e := mariadb.NewConnector(cfg)
	if e != nil {
		return nil, e
	}
	db := sql.OpenDB(connector)
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	if e = db.PingContext(ctx); e != nil {
		db.Close()
		return nil, e
	}
	return db, nil
}
