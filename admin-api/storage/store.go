package storage

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strconv"
	"strings"

	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

func NewStore(db *sql.DB, rdb *Redis) *Store {
	return &Store{db: db, rdb: rdb}
}

// ── Services ───────────────────────────────────────────────────────────────

func (s *Store) ListServices(limit, offset int) ([]Service, error) {
	rows, err := s.db.Query(`
		SELECT id, name, protocol, host, port, path, url, retries,
		       connect_timeout, read_timeout, write_timeout, enabled,
		       created_at, updated_at
		FROM services ORDER BY created_at DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Service
	for rows.Next() {
		var r Service
		var name, protocol, host, path, url sql.NullString
		var port, retries, connect, read, write sql.NullInt64
		var enabled sql.NullBool
		var created, updated sql.NullString
		if err := rows.Scan(&r.ID, &name, &protocol, &host, &port, &path, &url,
			&retries, &connect, &read, &write, &enabled, &created, &updated); err != nil {
			return nil, err
		}
		r.Name = name.String
		r.Protocol = protocol.String
		r.Host = host.String
		if port.Valid {
			r.Port = int(port.Int64)
		}
		r.Path = path.String
		r.URL = url.String
		if retries.Valid {
			r.Retries = int(retries.Int64)
		}
		if connect.Valid {
			r.ConnectTimeout = int(connect.Int64)
		}
		if read.Valid {
			r.ReadTimeout = int(read.Int64)
		}
		if write.Valid {
			r.WriteTimeout = int(write.Int64)
		}
		if enabled.Valid {
			r.Enabled = enabled.Bool
		}
		if created.Valid {
			r.CreatedAt = created.String
		}
		if updated.Valid {
			r.UpdatedAt = updated.String
		}
		out = append(out, r)
	}
	return out, nil
}

func (s *Store) CreateService(svc *Service) (*Service, error) {
	var id string
	err := s.db.QueryRow(`
		INSERT INTO services (name, protocol, host, port, path, url, retries,
			connect_timeout, read_timeout, write_timeout, enabled)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11)
		RETURNING id, created_at, updated_at`,
		svc.Name, orString(svc.Protocol, "http"), svc.Host, orInt(svc.Port, 80),
		svc.Path, svc.URL, orInt(svc.Retries, 5),
		orInt(svc.ConnectTimeout, 60000), orInt(svc.ReadTimeout, 60000),
		orInt(svc.WriteTimeout, 60000), orBool(svc.Enabled, true),
	).Scan(&id, &svc.CreatedAt, &svc.UpdatedAt)
	if err != nil {
		return nil, err
	}
	svc.ID = id
	return svc, nil
}

func (s *Store) GetService(id string) (*Service, error) {
	return s.getOneService(s.db.QueryRow(`
		SELECT id, name, protocol, host, port, path, url, retries,
		       connect_timeout, read_timeout, write_timeout, enabled,
		       created_at, updated_at FROM services WHERE id = $1`, id))
}

func (s *Store) UpdateService(id string, svc *Service) (*Service, error) {
	err := s.db.QueryRow(`
		UPDATE services SET
			name=$2, protocol=$3, host=$4, port=$5, path=$6, url=$7,
			retries=$8, connect_timeout=$9, read_timeout=$10,
			write_timeout=$11, enabled=$12, updated_at=NOW()
		WHERE id=$1 RETURNING updated_at`,
		id, svc.Name, orString(svc.Protocol, "http"), svc.Host,
		orInt(svc.Port, 80), svc.Path, svc.URL, orInt(svc.Retries, 5),
		orInt(svc.ConnectTimeout, 60000), orInt(svc.ReadTimeout, 60000),
		orInt(svc.WriteTimeout, 60000), orBool(svc.Enabled, true),
	).Scan(&svc.UpdatedAt)
	if err != nil {
		return nil, err
	}
	svc.ID = id
	return svc, nil
}

func (s *Store) DeleteService(id string) error {
	_, err := s.db.Exec("DELETE FROM services WHERE id=$1", id)
	return err
}

func (s *Store) getOneService(row *sql.Row) (*Service, error) {
	var r Service
	var name, protocol, host, path, url sql.NullString
	var port, retries, connect, read, write sql.NullInt64
	var enabled sql.NullBool
	var created, updated sql.NullString
	err := row.Scan(&r.ID, &name, &protocol, &host, &port, &path, &url,
		&retries, &connect, &read, &write, &enabled, &created, &updated)
	if err != nil {
		return nil, err
	}
	r.Name = name.String
	r.Protocol = protocol.String
	r.Host = host.String
	if port.Valid {
		r.Port = int(port.Int64)
	}
	r.Path = path.String
	r.URL = url.String
	if retries.Valid {
		r.Retries = int(retries.Int64)
	}
	if connect.Valid {
		r.ConnectTimeout = int(connect.Int64)
	}
	if read.Valid {
		r.ReadTimeout = int(read.Int64)
	}
	if write.Valid {
		r.WriteTimeout = int(write.Int64)
	}
	if enabled.Valid {
		r.Enabled = enabled.Bool
	}
	if created.Valid {
		r.CreatedAt = created.String
	}
	if updated.Valid {
		r.UpdatedAt = updated.String
	}
	return &r, nil
}

// ── Services (helper) ──────────────────────────────────────────────────────

func (s *Store) GetServiceByName(name string) (*Service, error) {
	row := s.db.QueryRow(`
		SELECT id, name, protocol, host, port, path, url, retries,
		       connect_timeout, read_timeout, write_timeout, enabled,
		       created_at, updated_at FROM services WHERE name=$1`, name)
	return s.getOneService(row)
}

// ── Routes ─────────────────────────────────────────────────────────────────

func (s *Store) ListRoutes(limit, offset int) ([]Route, error) {
	rows, err := s.db.Query(`
		SELECT id, name, service_id, protocols, hosts, paths, methods,
		       strip_path, preserve_host, regex_priority,
		       https_redirect_status_code, connection_timeout, enabled,
		       created_at, updated_at
		FROM routes ORDER BY created_at DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Route
	for rows.Next() {
		var r Route
		var name sql.NullString
		var serviceID sql.NullString
		var protocols, hosts, paths, methods []byte
		var stripPath, preserveHost sql.NullBool
		var regexPriority, httpsStatus, connTimeout sql.NullInt64
		var enabled sql.NullBool
		var created, updated sql.NullString
		if err := rows.Scan(&r.ID, &name, &serviceID, &protocols, &hosts,
			&paths, &methods, &stripPath, &preserveHost, &regexPriority,
			&httpsStatus, &connTimeout, &enabled, &created, &updated); err != nil {
			return nil, err
		}
		jsonScanSlice(&r.Protocols, protocols)
		jsonScanSlice(&r.Hosts, hosts)
		jsonScanSlice(&r.Paths, paths)
		jsonScanSlice(&r.Methods, methods)
		if name.Valid {
			r.Name = name.String
		}
		if serviceID.Valid {
			r.Service = &ServiceRef{ID: serviceID.String}
		}
		if stripPath.Valid {
			r.StripPath = stripPath.Bool
		}
		if preserveHost.Valid {
			r.PreserveHost = preserveHost.Bool
		}
		if regexPriority.Valid {
			r.RegexPriority = int(regexPriority.Int64)
		}
		if httpsStatus.Valid {
			r.HTTPSRedirectStatusCode = int(httpsStatus.Int64)
		}
		if connTimeout.Valid {
			r.ConnectionTimeout = int(connTimeout.Int64)
		}
		if enabled.Valid {
			r.Enabled = enabled.Bool
		}
		if created.Valid {
			r.CreatedAt = created.String
		}
		if updated.Valid {
			r.UpdatedAt = updated.String
		}
		out = append(out, r)
	}
	return out, nil
}

func (s *Store) CreateRoute(r *Route) (*Route, error) {
	protocols := orSlice(r.Protocols, []string{"http", "https"})
	hosts := orSlice(r.Hosts, []string{})
	paths := orSlice(r.Paths, []string{})
	methods := orSlice(r.Methods, []string{})

	err := s.db.QueryRow(`
		INSERT INTO routes (name, service_id, protocols, hosts, paths, methods,
			strip_path, preserve_host, regex_priority, https_redirect_status_code,
			connection_timeout, enabled)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		RETURNING id, created_at, updated_at`,
		r.Name, r.GetServiceID(), "{"+strings.Join(protocols,",")+"}", "{"+strings.Join(hosts,",")+"}", "{"+strings.Join(paths,",")+"}", "{"+strings.Join(methods,",")+"}",
		orBool(r.StripPath, true), orBool(r.PreserveHost, false),
		orInt(r.RegexPriority, 0), orInt(r.HTTPSRedirectStatusCode, 426),
		orInt(r.ConnectionTimeout, 60000), orBool(r.Enabled, true),
	).Scan(&r.ID, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (s *Store) GetRoute(id string) (*Route, error) {
	row := s.db.QueryRow(`
		SELECT id, name, service_id, protocols, hosts, paths, methods,
		       strip_path, preserve_host, regex_priority,
		       https_redirect_status_code, connection_timeout, enabled,
		       created_at, updated_at
		FROM routes WHERE id = $1`, id)
	var r Route
	var name, serviceID sql.NullString
	var protocols, hosts, paths, methods []byte
	var stripPath, preserveHost sql.NullBool
	var regexPriority, httpsStatus, connTimeout sql.NullInt64
	var enabled sql.NullBool
	var created, updated sql.NullString
	err := row.Scan(&r.ID, &name, &serviceID, &protocols, &hosts,
		&paths, &methods, &stripPath, &preserveHost, &regexPriority,
		&httpsStatus, &connTimeout, &enabled, &created, &updated)
	if err != nil {
		return nil, err
	}
r.Name = name.String
		if serviceID.Valid {
			r.Service = &ServiceRef{ID: serviceID.String}
		}
		jsonScanSlice(&r.Protocols, protocols)
		jsonScanSlice(&r.Hosts, hosts)
		jsonScanSlice(&r.Paths, paths)
		jsonScanSlice(&r.Methods, methods)
		if stripPath.Valid {
		r.StripPath = stripPath.Bool
	}
	if preserveHost.Valid {
		r.PreserveHost = preserveHost.Bool
	}
	if regexPriority.Valid {
		r.RegexPriority = int(regexPriority.Int64)
	}
	if httpsStatus.Valid {
		r.HTTPSRedirectStatusCode = int(httpsStatus.Int64)
	}
	if connTimeout.Valid {
		r.ConnectionTimeout = int(connTimeout.Int64)
	}
	if enabled.Valid {
		r.Enabled = enabled.Bool
	}
	if created.Valid {
		r.CreatedAt = created.String
	}
	if updated.Valid {
		r.UpdatedAt = updated.String
	}
	return &r, nil
}

func (s *Store) UpdateRoute(id string, r *Route) (*Route, error) {
	protocols := orSlice(r.Protocols, []string{"http", "https"})
	hosts := orSlice(r.Hosts, []string{})
	paths := orSlice(r.Paths, []string{})
	methods := orSlice(r.Methods, []string{})

	// Build UPDATE with only provided fields — avoid empty string for UUID
	setClauses := []string{"name=$2", "protocols=$3", "hosts=$4", "paths=$5", "methods=$6", "strip_path=$7", "preserve_host=$8", "regex_priority=$9", "https_redirect_status_code=$10", "connection_timeout=$11", "enabled=$12"}
	args := []interface{}{id, r.Name, "{" + strings.Join(protocols, ",") + "}", "{" + strings.Join(hosts, ",") + "}", "{" + strings.Join(paths, ",") + "}", "{" + strings.Join(methods, ",") + "}", orBool(r.StripPath, true), orBool(r.PreserveHost, false), orInt(r.RegexPriority, 0), orInt(r.HTTPSRedirectStatusCode, 426), orInt(r.ConnectionTimeout, 60000), orBool(r.Enabled, true)}
	if svcID := r.GetServiceID(); svcID != "" {
		setClauses = append([]string{"service_id=$13"}, setClauses...)
		args = append([]interface{}{svcID}, args...)
	}
	query := "UPDATE routes SET " + strings.Join(setClauses, ", ") + " WHERE id=$1 RETURNING updated_at"

	err := s.db.QueryRow(query, args...).Scan(&r.UpdatedAt)
	if err != nil {
		return nil, err
	}
	r.ID = id
	return r, nil
}

func (s *Store) DeleteRoute(id string) error {
	_, err := s.db.Exec("DELETE FROM routes WHERE id=$1", id)
	return err
}

// ── Upstreams ──────────────────────────────────────────────────────────────

func (s *Store) ListUpstreams(limit, offset int) ([]Upstream, error) {
	rows, err := s.db.Query(`
		SELECT id, name, algorithm, slots, healthchecks, enabled,
		       created_at, updated_at
		FROM upstreams ORDER BY created_at DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Upstream
	for rows.Next() {
		var u Upstream
		var name, algorithm, healthchecks sql.NullString
		var slots sql.NullInt64
		var enabled sql.NullBool
		var created, updated sql.NullString
		if err := rows.Scan(&u.ID, &name, &algorithm, &slots,
			&healthchecks, &enabled, &created, &updated); err != nil {
			return nil, err
		}
		u.Name = name.String
		u.Algorithm = orString(algorithm.String, "roundrobin")
		if slots.Valid {
			u.Slots = int(slots.Int64)
		}
		u.Healthchecks = healthchecks.String
		if enabled.Valid {
			u.Enabled = enabled.Bool
		}
		if created.Valid {
			u.CreatedAt = created.String
		}
		if updated.Valid {
			u.UpdatedAt = updated.String
		}
		out = append(out, u)
	}
	return out, nil
}

func (s *Store) CreateUpstream(u *Upstream) (*Upstream, error) {
	err := s.db.QueryRow(`
		INSERT INTO upstreams (name, algorithm, slots, healthchecks, enabled)
		VALUES ($1,$2,$3,$4,$5) RETURNING id, created_at, updated_at`,
		u.Name, orString(u.Algorithm, "roundrobin"), orInt(u.Slots, 10000),
		u.Healthchecks, orBool(u.Enabled, true),
	).Scan(&u.ID, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (s *Store) GetUpstream(id string) (*Upstream, error) {
	var u Upstream
	var name, algorithm, healthchecks sql.NullString
	var slots sql.NullInt64
	var enabled sql.NullBool
	var created, updated sql.NullString
	err := s.db.QueryRow(`
		SELECT id, name, algorithm, slots, healthchecks, enabled,
		       created_at, updated_at
		FROM upstreams WHERE id=$1`, id).Scan(
		&u.ID, &name, &algorithm, &slots, &healthchecks, &enabled, &created, &updated)
	if err != nil {
		return nil, err
	}
	u.Name = name.String
	u.Algorithm = algorithm.String
	if slots.Valid {
		u.Slots = int(slots.Int64)
	}
	u.Healthchecks = healthchecks.String
	if enabled.Valid {
		u.Enabled = enabled.Bool
	}
	if created.Valid {
		u.CreatedAt = created.String
	}
	if updated.Valid {
		u.UpdatedAt = updated.String
	}
	return &u, nil
}

func (s *Store) UpdateUpstream(id string, u *Upstream) (*Upstream, error) {
	err := s.db.QueryRow(`
		UPDATE upstreams SET name=$2, algorithm=$3, slots=$4,
			healthchecks=$5, enabled=$6, updated_at=NOW()
		WHERE id=$1 RETURNING updated_at`,
		id, u.Name, orString(u.Algorithm, "roundrobin"),
		orInt(u.Slots, 10000), u.Healthchecks, orBool(u.Enabled, true),
	).Scan(&u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	u.ID = id
	return u, nil
}

func (s *Store) DeleteUpstream(id string) error {
	_, err := s.db.Exec("DELETE FROM upstreams WHERE id=$1", id)
	return err
}

// ── Targets ─────────────────────────────────────────────────────────────────

func (s *Store) ListTargetsByUpstream(upstreamID string) ([]Target, error) {
	rows, err := s.db.Query(`
		SELECT id, target, weight, enabled, created_at
		FROM targets WHERE upstream_id=$1 ORDER BY created_at`, upstreamID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Target
	for rows.Next() {
		var t Target
		var target sql.NullString
		var weight sql.NullInt64
		var enabled sql.NullBool
		var created sql.NullString
		if err := rows.Scan(&t.ID, &target, &weight, &enabled, &created); err != nil {
			return nil, err
		}
		t.UpstreamID = upstreamID
		t.Target = target.String
		if weight.Valid {
			t.Weight = int(weight.Int64)
		}
		if enabled.Valid {
			t.Enabled = enabled.Bool
		}
		if created.Valid {
			t.CreatedAt = created.String
		}
		out = append(out, t)
	}
	return out, nil
}

func (s *Store) CreateTarget(t *Target) (*Target, error) {
	err := s.db.QueryRow(`
		INSERT INTO targets (upstream_id, target, weight, enabled)
		VALUES ($1,$2,$3,$4) RETURNING id, created_at`,
		t.UpstreamID, t.Target, orInt(t.Weight, 100), orBool(t.Enabled, true),
	).Scan(&t.ID, &t.CreatedAt)
	if err != nil {
		return nil, err
	}
	return t, nil
}

func (s *Store) UpdateTarget(upstreamID, targetID string, t *Target) (*Target, error) {
	err := s.db.QueryRow(`
		UPDATE targets SET target=$2, weight=$3, enabled=$4
		WHERE id=$1 AND upstream_id=$5 RETURNING id`,
		targetID, t.Target, orInt(t.Weight, 100), orBool(t.Enabled, true), upstreamID,
	).Scan(&t.ID)
	if err != nil {
		return nil, err
	}
	t.UpstreamID = upstreamID
	return t, nil
}

func (s *Store) DeleteTarget(upstreamID, targetID string) error {
	_, err := s.db.Exec("DELETE FROM targets WHERE id=$1 AND upstream_id=$2", targetID, upstreamID)
	return err
}

// ── Consumers ───────────────────────────────────────────────────────────────

func (s *Store) ListConsumers(limit, offset int) ([]Consumer, error) {
	rows, err := s.db.Query(`
		SELECT id, username, custom_id, enabled, created_at, updated_at
		FROM consumers ORDER BY created_at DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Consumer
	for rows.Next() {
		var c Consumer
		var username, customID sql.NullString
		var enabled sql.NullBool
		var created, updated sql.NullString
		if err := rows.Scan(&c.ID, &username, &customID, &enabled, &created, &updated); err != nil {
			return nil, err
		}
		c.Username = username.String
		c.CustomID = customID.String
		if enabled.Valid {
			c.Enabled = enabled.Bool
		}
		if created.Valid {
			c.CreatedAt = created.String
		}
		if updated.Valid {
			c.UpdatedAt = updated.String
		}
		out = append(out, c)
	}
	return out, nil
}

func (s *Store) CreateConsumer(c *Consumer) (*Consumer, error) {
	err := s.db.QueryRow(`
		INSERT INTO consumers (username, custom_id, enabled)
		VALUES ($1,$2,$3) RETURNING id, created_at, updated_at`,
		c.Username, c.CustomID, orBool(c.Enabled, true),
	).Scan(&c.ID, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func (s *Store) GetConsumer(id string) (*Consumer, error) {
	var c Consumer
	var username, customID sql.NullString
	var enabled sql.NullBool
	var created, updated sql.NullString
	err := s.db.QueryRow(`
		SELECT id, username, custom_id, enabled, created_at, updated_at
		FROM consumers WHERE id=$1`, id).Scan(
		&c.ID, &username, &customID, &enabled, &created, &updated)
	if err != nil {
		return nil, err
	}
	c.Username = username.String
	c.CustomID = customID.String
	if enabled.Valid {
		c.Enabled = enabled.Bool
	}
	if created.Valid {
		c.CreatedAt = created.String
	}
	if updated.Valid {
		c.UpdatedAt = updated.String
	}
	return &c, nil
}

func (s *Store) UpdateConsumer(id string, c *Consumer) (*Consumer, error) {
	err := s.db.QueryRow(`
		UPDATE consumers SET username=$2, custom_id=$3, enabled=$4, updated_at=NOW()
		WHERE id=$1 RETURNING updated_at`,
		id, c.Username, c.CustomID, orBool(c.Enabled, true),
	).Scan(&c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	c.ID = id
	return c, nil
}

func (s *Store) DeleteConsumer(id string) error {
	_, err := s.db.Exec("DELETE FROM consumers WHERE id=$1", id)
	return err
}

// ── Consumer Credentials ───────────────────────────────────────────────────

// ListConsumerCredentials returns all credentials for a consumer of a given type
func (s *Store) ListConsumerCredentials(consumerID, credentialType string) ([]ConsumerCredential, error) {
	rows, err := s.db.Query(`
		SELECT id, consumer_id, credential_type, key, secret, enabled, created_at
		FROM consumer_credentials
		WHERE consumer_id=$1 AND credential_type=$2
		ORDER BY created_at DESC`,
		consumerID, credentialType)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ConsumerCredential
	for rows.Next() {
		var c ConsumerCredential
		var secret sql.NullString
		var created sql.NullString
		if err := rows.Scan(&c.ID, &c.ConsumerID, &c.CredentialType, &c.Key, &secret, &c.Enabled, &created); err != nil {
			return nil, err
		}
		if secret.Valid {
			c.Secret = secret.String
		}
		if created.Valid {
			c.CreatedAt = created.String
		}
		out = append(out, c)
	}
	return out, nil
}

// CreateConsumerCredential creates a new credential for a consumer
func (s *Store) CreateConsumerCredential(c *ConsumerCredential) (*ConsumerCredential, error) {
	err := s.db.QueryRow(`
		INSERT INTO consumer_credentials (consumer_id, credential_type, key, secret, enabled)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, created_at`,
		c.ConsumerID, c.CredentialType, c.Key, c.Secret, orBool(c.Enabled, true),
	).Scan(&c.ID, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	return c, nil
}

// GetConsumerCredentialByKey looks up a credential by type and key (for auth middleware)
func (s *Store) GetConsumerCredentialByKey(credentialType, key string) (*ConsumerCredential, error) {
	var c ConsumerCredential
	var secret sql.NullString
	var created sql.NullString
	err := s.db.QueryRow(`
		SELECT id, consumer_id, credential_type, key, secret, enabled, created_at
		FROM consumer_credentials
		WHERE credential_type=$1 AND key=$2 AND enabled=true`,
		credentialType, key,
	).Scan(&c.ID, &c.ConsumerID, &c.CredentialType, &c.Key, &secret, &c.Enabled, &created)
	if err != nil {
		return nil, err
	}
	if secret.Valid {
		c.Secret = secret.String
	}
	if created.Valid {
		c.CreatedAt = created.String
	}
	return &c, nil
}

// DeleteConsumerCredential deletes a specific credential
func (s *Store) DeleteConsumerCredential(consumerID, credentialType, credentialID string) error {
	result, err := s.db.Exec(
		`DELETE FROM consumer_credentials WHERE id=$1 AND consumer_id=$2 AND credential_type=$3`,
		credentialID, consumerID, credentialType,
	)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
}

// ── Plugins ────────────────────────────────────────────────────────────────

func (s *Store) ListPlugins(limit, offset int) ([]Plugin, error) {
	rows, err := s.db.Query(`
		SELECT id, name, route_id, service_id, consumer_id, config, enabled,
		       created_at, updated_at
		FROM plugins ORDER BY created_at DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Plugin
	for rows.Next() {
		var p Plugin
		var name sql.NullString
		var routeID, serviceID, consumerID sql.NullString
		var config []byte
		var enabled sql.NullBool
		var created, updated sql.NullString
		if err := rows.Scan(&p.ID, &name, &routeID, &serviceID, &consumerID,
			&config, &enabled, &created, &updated); err != nil {
			return nil, err
		}
		p.Name = name.String
		if routeID.Valid {
			p.Route =&PluginScope{ID: routeID.String}
		}
		if serviceID.Valid {
			p.Service = &PluginScope{ID: serviceID.String}
		}
		if consumerID.Valid {
			p.Consumer = &PluginScope{ID: consumerID.String}
		}
		if len(config) > 0 {
			p.Config = json.RawMessage(config)
		}
		if enabled.Valid {
			p.Enabled = enabled.Bool
		}
		if created.Valid {
			p.CreatedAt = created.String
		}
		if updated.Valid {
			p.UpdatedAt = updated.String
		}
		out = append(out, p)
	}
	return out, nil
}

func (s *Store) CreatePlugin(p *Plugin) (*Plugin, error) {
	configJSON, _ := json.Marshal(p.Config)
	var routeID, serviceID, consumerID *string
	if p.Route != nil {
		routeID = &p.Route.ID
	}
	if p.Service != nil {
		serviceID = &p.Service.ID
	}
	if p.Consumer != nil {
		consumerID = &p.Consumer.ID
	}
	err := s.db.QueryRow(`
		INSERT INTO plugins (name, route_id, service_id, consumer_id, config, enabled)
		VALUES ($1,$2,$3,$4,$5,$6) RETURNING id, created_at, updated_at`,
		p.Name, routeID, serviceID, consumerID, configJSON, orBool(p.Enabled, true),
	).Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return p, nil
}

func (s *Store) GetPlugin(id string) (*Plugin, error) {
	var p Plugin
	var name sql.NullString
	var routeID, serviceID, consumerID sql.NullString
	var config []byte
	var enabled sql.NullBool
	var created, updated sql.NullString
	err := s.db.QueryRow(`
		SELECT id, name, route_id, service_id, consumer_id, config, enabled,
		       created_at, updated_at FROM plugins WHERE id=$1`, id).Scan(
		&p.ID, &name, &routeID, &serviceID, &consumerID,
		&config, &enabled, &created, &updated)
	if err != nil {
		return nil, err
	}
	p.Name = name.String
	if routeID.Valid {
		p.Route =&PluginScope{ID: routeID.String}
	}
	if serviceID.Valid {
		p.Service = &PluginScope{ID: serviceID.String}
	}
	if consumerID.Valid {
		p.Consumer = &PluginScope{ID: consumerID.String}
	}
	if len(config) > 0 {
		p.Config = json.RawMessage(config)
	}
	if enabled.Valid {
		p.Enabled = enabled.Bool
	}
	if created.Valid {
		p.CreatedAt = created.String
	}
	if updated.Valid {
		p.UpdatedAt = updated.String
	}
	return &p, nil
}

func (s *Store) UpdatePlugin(id string, p *Plugin) (*Plugin, error) {
	configJSON, _ := json.Marshal(p.Config)
	var routeID, serviceID, consumerID *string
	if p.Route != nil {
		routeID = &p.Route.ID
	}
	if p.Service != nil {
		serviceID = &p.Service.ID
	}
	if p.Consumer != nil {
		consumerID = &p.Consumer.ID
	}
	err := s.db.QueryRow(`
		UPDATE plugins SET name=$2, route_id=$3, service_id=$4, consumer_id=$5,
			config=$6, enabled=$7, updated_at=NOW()
		WHERE id=$1 RETURNING updated_at`,
		id, p.Name, routeID, serviceID, consumerID, configJSON, orBool(p.Enabled, true),
	).Scan(&p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	p.ID = id
	return p, nil
}

func (s *Store) DeletePlugin(id string) error {
	_, err := s.db.Exec("DELETE FROM plugins WHERE id=$1", id)
	return err
}

// ── Workspaces ─────────────────────────────────────────────────────────────

func (s *Store) ListWorkspaces() ([]Workspace, error) {
	rows, err := s.db.Query(`SELECT id, name, created_at FROM workspaces`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Workspace
	for rows.Next() {
		var w Workspace
		var name sql.NullString
		var created sql.NullString
		if err := rows.Scan(&w.ID, &name, &created); err != nil {
			return nil, err
		}
		w.Name = name.String
		if created.Valid {
			w.CreatedAt = created.String
		}
		out = append(out, w)
	}
	return out, nil
}

func (s *Store) CreateWorkspace(w *Workspace) (*Workspace, error) {
	err := s.db.QueryRow(`
		INSERT INTO workspaces (name) VALUES ($1) RETURNING id, created_at`,
		w.Name).Scan(&w.ID, &w.CreatedAt)
	if err != nil {
		return nil, err
	}
	return w, nil
}

func (s *Store) GetWorkspace(id string) (*Workspace, error) {
	var w Workspace
	var name sql.NullString
	var created sql.NullString
	err := s.db.QueryRow(`SELECT id, name, created_at FROM workspaces WHERE id=$1`,
		id).Scan(&w.ID, &name, &created)
	if err != nil {
		return nil, err
	}
	w.Name = name.String
	if created.Valid {
		w.CreatedAt = created.String
	}
	return &w, nil
}

func (s *Store) UpdateWorkspace(id string, w *Workspace) (*Workspace, error) {
	err := s.db.QueryRow(`
		UPDATE workspaces SET name=$1 WHERE id=$2
		RETURNING id, name, created_at`,
		w.Name, id,
	).Scan(&w.ID, &w.Name, &w.CreatedAt)
	if err != nil {
		return nil, err
	}
	return w, nil
}

func (s *Store) DeleteWorkspace(id string) error {
	_, err := s.db.Exec(`DELETE FROM workspaces WHERE id=$1`, id)
	return err
}

// ── User Workspaces ─────────────────────────────────────────────────────────

// ListUserWorkspaces returns all workspaces a user has access to (direct assignment)
// Returns WorkspaceUserAssignment objects so the frontend gets role information
func (s *Store) ListUserWorkspaces(userID string) ([]WorkspaceUserAssignment, error) {
	rows, err := s.db.Query(`
		SELECT w.id, w.name, w.created_at, uw.role
		FROM user_workspaces uw
		JOIN workspaces w ON w.id = uw.workspace_id
		WHERE uw.user_id = $1
		ORDER BY w.name`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WorkspaceUserAssignment
	if out == nil {
		out = []WorkspaceUserAssignment{}
	}
	for rows.Next() {
		var w WorkspaceUserAssignment
		var name sql.NullString
		var created sql.NullString
		if err := rows.Scan(&w.WorkspaceID, &name, &created, &w.Role); err != nil {
			return nil, err
		}
		w.Username = name.String // reuse name field as workspace name for frontend
		if created.Valid {
			w.AssignedAt = created.String
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// GetUserWorkspaceRole returns the user's role in a workspace (empty string if no access)
func (s *Store) GetUserWorkspaceRole(userID, workspaceID string) (string, error) {
	var role string
	err := s.db.QueryRow(`
		SELECT role FROM user_workspaces
		WHERE user_id = $1 AND workspace_id = $2`, userID, workspaceID).Scan(&role)
	if err == sql.ErrNoRows {
		return "", nil
	}
	return role, err
}

// SetUserWorkspace sets a user's role in a workspace (upsert)
func (s *Store) SetUserWorkspace(userID, workspaceID, role string) error {
	_, err := s.db.Exec(`
		INSERT INTO user_workspaces (user_id, workspace_id, role)
		VALUES ($1, $2, $3)
		ON CONFLICT (user_id, workspace_id) DO UPDATE SET role = $3`,
		userID, workspaceID, role)
	return err
}

// RemoveUserWorkspace removes a user's access to a workspace
func (s *Store) RemoveUserWorkspace(userID, workspaceID string) error {
	_, err := s.db.Exec(`DELETE FROM user_workspaces WHERE user_id=$1 AND workspace_id=$2`,
		userID, workspaceID)
	return err
}

// ListWorkspaceUsers returns all users assigned to a workspace with their roles
func (s *Store) ListWorkspaceUsers(workspaceID string) ([]WorkspaceUserAssignment, error) {
	rows, err := s.db.Query(`
		SELECT u.id, u.username, u.display_name, u.email, uw.role, uw.created_at
		FROM user_workspaces uw
		JOIN users u ON u.id = uw.user_id
		WHERE uw.workspace_id = $1
		ORDER BY u.username`, workspaceID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []WorkspaceUserAssignment
	if out == nil {
		out = []WorkspaceUserAssignment{}
	}
	for rows.Next() {
		var a WorkspaceUserAssignment
		var displayName, email sql.NullString
		var createdAt sql.NullString
		if err := rows.Scan(&a.UserID, &a.Username, &displayName, &email, &a.Role, &createdAt); err != nil {
			return nil, err
		}
		a.DisplayName = displayName.String
		a.Email = email.String
		if createdAt.Valid {
			a.AssignedAt = createdAt.String
		}
		out = append(out, a)
	}
	return out, rows.Err()
}

// ── Helpers ────────────────────────────────────────────────────────────────

func jsonScanSlice(out *[]string, data []byte) {
	if len(data) == 0 || string(data) == "null" {
		*out = []string{}
		return
	}
	if err := json.Unmarshal(data, out); err != nil {
		// Fallback: PostgreSQL TEXT[] format like {http,https}
		s := string(data)
		s = strings.TrimPrefix(s, "{")
		s = strings.TrimSuffix(s, "}")
		if s == "" {
			*out = []string{}
			return
		}
		*out = strings.Split(s, ",")
	}
}

func nullString(s string) interface{} {
	if s == "" {
		return nil
	}
	return s
}

func stringVal(p *string) string {
	if p == nil {
		return ""
	}
	return *p
}

func orString(s, def string) string {
	if s == "" {
		return def
	}
	return s
}

func orInt(v int, def int) int {
	if v == 0 {
		return def
	}
	return v
}

func orBool(v bool, def bool) bool {
	return v || def
}

func orSlice(v []string, def []string) []string {
	if len(v) == 0 {
		return def
	}
	return v
}

func firstLine(s string) string {
	return strings.Split(s, "\n")[0]
}

// User methods

func (s *Store) GetUserByUsername(username string) (*User, error) {
	row := s.db.QueryRow(`SELECT id, username, password_hash, display_name, email, role, enabled, created_at, updated_at FROM users WHERE username = $1 AND enabled = true`, username)
	var u User
	var displayName, email sql.NullString
	err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &displayName, &email, &u.Role, &u.Enabled, &u.CreatedAt, &u.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	u.DisplayName = displayName.String
	u.Email = email.String
	return &u, nil
}

func (s *Store) CreateUser(u *User) (*User, error) {
	err := s.db.QueryRow(`INSERT INTO users (username, password_hash, display_name, email, role) VALUES ($1, $2, $3, $4, $5) RETURNING id, created_at, updated_at`,
		u.Username, u.PasswordHash, u.DisplayName, u.Email, u.Role).Scan(&u.ID, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (s *Store) ListUsers() ([]User, error) {
	rows, err := s.db.Query(`SELECT id, username, password_hash, display_name, email, role, enabled, created_at, updated_at FROM users ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []User
	for rows.Next() {
		var u User
		var displayName, email sql.NullString
		if err := rows.Scan(&u.ID, &u.Username, &u.PasswordHash, &displayName, &email, &u.Role, &u.Enabled, &u.CreatedAt, &u.UpdatedAt); err != nil {
			return nil, err
		}
		u.DisplayName = displayName.String
		u.Email = email.String
		groups, _ := s.GetUserGroups(u.ID)
		u.Groups = groups
		out = append(out, u)
	}
	return out, rows.Err()
}

func (s *Store) GetUser(id string) (*User, error) {
	row := s.db.QueryRow(`SELECT id, username, password_hash, display_name, email, role, enabled, created_at, updated_at FROM users WHERE id = $1`, id)
	var u User
	if err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.DisplayName, &u.Email, &u.Role, &u.Enabled, &u.CreatedAt, &u.UpdatedAt); err == sql.ErrNoRows {
		return nil, nil
	} else if err != nil {
		return nil, err
	}
	groups, _ := s.GetUserGroups(u.ID)
	u.Groups = groups
	return &u, nil
}

func (s *Store) UpdateUser(id string, u *User) error {
	_, err := s.db.Exec(`UPDATE users SET display_name=$1, email=$2, role=$3, enabled=$4, updated_at=NOW() WHERE id=$5`,
		u.DisplayName, u.Email, u.Role, u.Enabled, id)
	return err
}

func (s *Store) DeleteUser(id string) error {
	_, err := s.db.Exec(`DELETE FROM users WHERE id=$1`, id)
	return err
}

// GetUserGroups returns the AuthGroups that a user belongs to
func (s *Store) GetUserGroups(userID string) ([]UserGroupRef, error) {
	rows, err := s.db.Query(`
		SELECT ag.name, ag.label
		FROM auth_groups ag
		JOIN user_auth_groups uag ON uag.group_id = ag.id
		WHERE uag.user_id = $1
		ORDER BY ag.label
	`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var groups []UserGroupRef
	for rows.Next() {
		var g UserGroupRef
		if err := rows.Scan(&g.Name, &g.Label); err != nil {
			return nil, err
		}
		groups = append(groups, g)
	}
	return groups, rows.Err()
}

func (s *Store) UpdateUserPassword(id, passwordHash string) error {
	_, err := s.db.Exec(`UPDATE users SET password_hash=$1, updated_at=NOW() WHERE id=$2`, passwordHash, id)
	return err
}

// ── Auth Groups ────────────────────────────────────────────────────────────

func (s *Store) ListAuthGroups() ([]AuthGroup, error) {
	rows, err := s.db.Query(`SELECT id, name, label, description, permissions, created_at, updated_at FROM auth_groups ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var groups []AuthGroup
	for rows.Next() {
		var g AuthGroup
		var label, desc sql.NullString
		var perms []byte
		var createdAt, updatedAt sql.NullString
		if err := rows.Scan(&g.ID, &g.Name, &label, &desc, &perms, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		if label.Valid {
			g.Label = label.String
		}
		if desc.Valid {
			g.Description = desc.String
		}
		if perms != nil {
			json.Unmarshal(perms, &g.Permissions)
		}
		if createdAt.Valid {
			g.CreatedAt = createdAt.String
		}
		if updatedAt.Valid {
			g.UpdatedAt = updatedAt.String
		}
		groups = append(groups, g)
	}
	return groups, nil
}

func (s *Store) CreateAuthGroup(g *AuthGroup) (*AuthGroup, error) {
	permsJSON, _ := json.Marshal(g.Permissions)
	var outID string
	err := s.db.QueryRow(
		`INSERT INTO auth_groups (name, label, description, permissions) VALUES ($1,$2,$3,$4) RETURNING id, created_at, updated_at`,
		g.Name, nullString(g.Label), nullString(g.Description), permsJSON,
	).Scan(&outID, &g.CreatedAt, &g.UpdatedAt)
	if err != nil {
		return nil, err
	}
	g.ID = outID
	return g, nil
}

func (s *Store) GetAuthGroup(id string) (*AuthGroup, error) {
	var g AuthGroup
	var label, desc sql.NullString
	var perms []byte
	var createdAt, updatedAt sql.NullString
	err := s.db.QueryRow(
		`SELECT id, name, label, description, permissions, created_at, updated_at FROM auth_groups WHERE id=$1`,
		id,
	).Scan(&g.ID, &g.Name, &label, &desc, &perms, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	if label.Valid {
		g.Label = label.String
	}
	if desc.Valid {
		g.Description = desc.String
	}
	if perms != nil {
		json.Unmarshal(perms, &g.Permissions)
	}
	if createdAt.Valid {
		g.CreatedAt = createdAt.String
	}
	if updatedAt.Valid {
		g.UpdatedAt = updatedAt.String
	}
	return &g, nil
}

func (s *Store) UpdateAuthGroup(id string, g *AuthGroup) (*AuthGroup, error) {
	permsJSON, _ := json.Marshal(g.Permissions)
	err := s.db.QueryRow(
		`UPDATE auth_groups SET name=$2, label=$3, description=$4, permissions=$5, updated_at=NOW() WHERE id=$1 RETURNING updated_at`,
		id, g.Name, nullString(g.Label), nullString(g.Description), permsJSON,
	).Scan(&g.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return g, nil
}

func (s *Store) DeleteAuthGroup(id string) error {
	_, err := s.db.Exec(`DELETE FROM auth_groups WHERE id=$1`, id)
	return err
}

func (s *Store) GetAuthGroupByName(name string) (*AuthGroup, error) {
	var g AuthGroup
	var label, desc sql.NullString
	var perms []byte
	var createdAt, updatedAt sql.NullString
	err := s.db.QueryRow(
		`SELECT id, name, label, description, permissions, created_at, updated_at FROM auth_groups WHERE name=$1`,
		name,
	).Scan(&g.ID, &g.Name, &label, &desc, &perms, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	if label.Valid {
		g.Label = label.String
	}
	if desc.Valid {
		g.Description = desc.String
	}
	if perms != nil {
		json.Unmarshal(perms, &g.Permissions)
	}
	if createdAt.Valid {
		g.CreatedAt = createdAt.String
	}
	if updatedAt.Valid {
		g.UpdatedAt = updatedAt.String
	}
	return &g, nil
}

// ── Auth Group Members ───────────────────────────────────────────────────────

// ListGroupMembers returns all user IDs in an auth group
func (s *Store) ListGroupMembers(groupID string) ([]string, error) {
	rows, err := s.db.Query(`SELECT user_id FROM user_auth_groups WHERE group_id=$1`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, nil
}

// AddUserToGroup adds a user to an auth group
func (s *Store) AddUserToGroup(userID, groupID string) error {
	_, err := s.db.Exec(
		`INSERT INTO user_auth_groups (user_id, group_id) VALUES ($1,$2) ON CONFLICT DO NOTHING`,
		userID, groupID,
	)
	return err
}

// RemoveUserFromGroup removes a user from an auth group
func (s *Store) RemoveUserFromGroup(userID, groupID string) error {
	_, err := s.db.Exec(`DELETE FROM user_auth_groups WHERE user_id=$1 AND group_id=$2`, userID, groupID)
	return err
}

// SetGroupMembers replaces all members of a group with the given user IDs
func (s *Store) SetGroupMembers(groupID string, userIDs []string) error {
	tx, err := s.db.Begin()
	if err != nil {
		return err
	}
	defer tx.Rollback()

	// Remove all existing members
	if _, err := tx.Exec(`DELETE FROM user_auth_groups WHERE group_id=$1`, groupID); err != nil {
		return err
	}
	// Add new members
	for _, uid := range userIDs {
		if _, err := tx.Exec(`INSERT INTO user_auth_groups (user_id, group_id) VALUES ($1,$2)`, uid, groupID); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// ── Resources ──────────────────────────────────────────────────────────────

func (s *Store) ListResources() ([]Resource, error) {
	rows, err := s.db.Query(`SELECT id, name, path, type FROM resources ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var resources []Resource
	for rows.Next() {
		var r Resource
		var typ sql.NullString
		if err := rows.Scan(&r.ID, &r.Name, &r.Path, &typ); err != nil {
			return nil, err
		}
		if typ.Valid {
			r.Type = typ.String
		}
		resources = append(resources, r)
	}
	return resources, nil
}

// ── Audit Log ──────────────────────────────────────────────────────────────

func (s *Store) ListAuditLogs(limit, offset int) ([]AuditLog, error) {
	rows, err := s.db.Query(
		`SELECT id, audit_type, target_type, target_id, actor_user_id, actor_username, description, created_at
		 FROM audit_logs ORDER BY created_at DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var logs []AuditLog
	for rows.Next() {
		var l AuditLog
		var actorUID, actorUname sql.NullString
		if err := rows.Scan(&l.ID, &l.AuditType, &l.TargetType, &l.TargetID, &actorUID, &actorUname, &l.Description, &l.CreatedAt); err != nil {
			return nil, err
		}
		if actorUID.Valid {
			l.ActorUserID = actorUID.String
		}
		if actorUname.Valid {
			l.ActorUsername = actorUname.String
		}
		logs = append(logs, l)
	}
	return logs, nil
}

func (s *Store) CreateAuditLog(l *AuditLog) error {
	_, err := s.db.Exec(
		`INSERT INTO audit_logs (audit_type, target_type, target_id, actor_user_id, actor_username, description)
		 VALUES ($1,$2,$3,$4,$5,$6)`,
		l.AuditType, l.TargetType, l.TargetID, nullString(l.ActorUserID), nullString(l.ActorUsername), l.Description,
	)
	return err
}

// ── Alert Rules ────────────────────────────────────────────────────────────

func (s *Store) ListAlertRules() ([]AlertRule, error) {
	rows, err := s.db.Query(
		`SELECT id, name, description, metric_type, service_name, threshold_value, operator,
		        duration_seconds, enabled, notification_channels, slack_webhook_url,
		        email_webhook_url, discord_webhook_url, alert_suppress_seconds, created_at, updated_at
		 FROM alert_rules ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var rules []AlertRule
	for rows.Next() {
		var r AlertRule
		var desc, svcName, notifCh, slackURL, emailURL, discordURL sql.NullString
		var createdAt, updatedAt sql.NullString
		if err := rows.Scan(&r.ID, &r.Name, &desc, &r.MetricType, &svcName, &r.ThresholdValue, &r.Operator,
			&r.DurationSeconds, &r.Enabled, &notifCh, &slackURL, &emailURL, &discordURL,
			&r.AlertSuppressSeconds, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		if desc.Valid {
			r.Description = desc.String
		}
		if svcName.Valid {
			r.ServiceName = svcName.String
		}
		if notifCh.Valid {
			r.NotificationChannels = notifCh.String
		}
		if slackURL.Valid {
			r.SlackWebhookURL = slackURL.String
		}
		if emailURL.Valid {
			r.EmailWebhookURL = emailURL.String
		}
		if discordURL.Valid {
			r.DiscordWebhookURL = discordURL.String
		}
		if createdAt.Valid {
			r.CreatedAt = createdAt.String
		}
		if updatedAt.Valid {
			r.UpdatedAt = updatedAt.String
		}
		rules = append(rules, r)
	}
	return rules, nil
}

func (s *Store) CreateAlertRule(r *AlertRule) (*AlertRule, error) {
	var outID int64
	err := s.db.QueryRow(
		`INSERT INTO alert_rules (name, description, metric_type, service_name, threshold_value, operator,
		 duration_seconds, enabled, notification_channels, slack_webhook_url, email_webhook_url,
		 discord_webhook_url, alert_suppress_seconds) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		 RETURNING id, created_at, updated_at`,
		r.Name, nullString(r.Description), r.MetricType, nullString(r.ServiceName), r.ThresholdValue,
		r.Operator, r.DurationSeconds, r.Enabled, nullString(r.NotificationChannels),
		nullString(r.SlackWebhookURL), nullString(r.EmailWebhookURL), nullString(r.DiscordWebhookURL),
		r.AlertSuppressSeconds,
	).Scan(&outID, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return nil, err
	}
	r.ID = outID
	return r, nil
}

func (s *Store) GetAlertRule(id string) (*AlertRule, error) {
	var r AlertRule
	var desc, svcName, notifCh, slackURL, emailURL, discordURL sql.NullString
	var createdAt, updatedAt sql.NullString
	err := s.db.QueryRow(
		`SELECT id, name, description, metric_type, service_name, threshold_value, operator,
		        duration_seconds, enabled, notification_channels, slack_webhook_url,
		        email_webhook_url, discord_webhook_url, alert_suppress_seconds, created_at, updated_at
		 FROM alert_rules WHERE id=$1`, id,
	).Scan(&r.ID, &r.Name, &desc, &r.MetricType, &svcName, &r.ThresholdValue, &r.Operator,
		&r.DurationSeconds, &r.Enabled, &notifCh, &slackURL, &emailURL, &discordURL,
		&r.AlertSuppressSeconds, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	if desc.Valid {
		r.Description = desc.String
	}
	if svcName.Valid {
		r.ServiceName = svcName.String
	}
	if notifCh.Valid {
		r.NotificationChannels = notifCh.String
	}
	if slackURL.Valid {
		r.SlackWebhookURL = slackURL.String
	}
	if emailURL.Valid {
		r.EmailWebhookURL = emailURL.String
	}
	if discordURL.Valid {
		r.DiscordWebhookURL = discordURL.String
	}
	if createdAt.Valid {
		r.CreatedAt = createdAt.String
	}
	if updatedAt.Valid {
		r.UpdatedAt = updatedAt.String
	}
	return &r, nil
}

func (s *Store) UpdateAlertRule(id string, r *AlertRule) (*AlertRule, error) {
	err := s.db.QueryRow(
		`UPDATE alert_rules SET name=$2, description=$3, metric_type=$4, service_name=$5, threshold_value=$6,
		 operator=$7, duration_seconds=$8, enabled=$9, notification_channels=$10, slack_webhook_url=$11,
		 email_webhook_url=$12, discord_webhook_url=$13, alert_suppress_seconds=$14, updated_at=NOW()
		 WHERE id=$1 RETURNING updated_at`,
		id, r.Name, nullString(r.Description), r.MetricType, nullString(r.ServiceName), r.ThresholdValue,
		r.Operator, r.DurationSeconds, r.Enabled, nullString(r.NotificationChannels),
		nullString(r.SlackWebhookURL), nullString(r.EmailWebhookURL), nullString(r.DiscordWebhookURL),
		r.AlertSuppressSeconds,
	).Scan(&r.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return r, nil
}

func (s *Store) DeleteAlertRule(id string) error {
	_, err := s.db.Exec(`DELETE FROM alert_rules WHERE id=$1`, id)
	return err
}

// ── API Key Requests ───────────────────────────────────────────────────────

func (s *Store) ListAPIKeyRequests() ([]APIKeyRequest, error) {
	rows, err := s.db.Query(
		`SELECT id, key_name, consumer_name, description, status, applicant_user_id, applicant_username,
		        reviewed_by, reviewed_at, created_at, updated_at
		 FROM api_key_requests ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var reqs []APIKeyRequest
	for rows.Next() {
		var r APIKeyRequest
		var consumerName, desc, reviewedBy, reviewedAt sql.NullString
		var createdAt, updatedAt sql.NullString
		var applicantUserID, applicantUsername sql.NullString
		if err := rows.Scan(&r.ID, &r.KeyName, &consumerName, &desc, &r.Status,
			&applicantUserID, &applicantUsername, &reviewedBy, &reviewedAt, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		if applicantUserID.Valid {
			r.ApplicantUserID = applicantUserID.String
		}
		if applicantUsername.Valid {
			r.ApplicantUsername = applicantUsername.String
		}
		if consumerName.Valid {
			r.ConsumerName = consumerName.String
		}
		if desc.Valid {
			r.Description = desc.String
		}
		if reviewedBy.Valid {
			r.ReviewedBy = reviewedBy.String
		}
		if reviewedAt.Valid {
			r.ReviewedAt = reviewedAt.String
		}
		if createdAt.Valid {
			r.CreatedAt = createdAt.String
		}
		if updatedAt.Valid {
			r.UpdatedAt = updatedAt.String
		}
		reqs = append(reqs, r)
	}
	return reqs, nil
}

func (s *Store) CreateAPIKeyRequest(r *APIKeyRequest) (*APIKeyRequest, error) {
	var outID int64
	err := s.db.QueryRow(
		`INSERT INTO api_key_requests (key_name, consumer_name, description, status, applicant_user_id, applicant_username)
		 VALUES ($1,$2,$3,$4,$5,$6) RETURNING id, created_at, updated_at`,
		r.KeyName, nullString(r.ConsumerName), nullString(r.Description), r.Status,
		nullString(r.ApplicantUserID), nullString(r.ApplicantUsername),
	).Scan(&outID, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return nil, err
	}
	r.ID = outID
	return r, nil
}

func (s *Store) GetAPIKeyRequest(id string) (*APIKeyRequest, error) {
	intID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return nil, err
	}
	var r APIKeyRequest
	var consumerName, desc, reviewedBy, reviewedAt sql.NullString
	var createdAt, updatedAt sql.NullString
	var applicantUserID, applicantUsername sql.NullString
	err = s.db.QueryRow(
		`SELECT id, key_name, consumer_name, description, status, applicant_user_id, applicant_username,
		        reviewed_by, reviewed_at, created_at, updated_at
		 FROM api_key_requests WHERE id=$1`, intID,
	).Scan(&r.ID, &r.KeyName, &consumerName, &desc, &r.Status,
		&applicantUserID, &applicantUsername, &reviewedBy, &reviewedAt, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	if applicantUserID.Valid {
		r.ApplicantUserID = applicantUserID.String
	}
	if applicantUsername.Valid {
		r.ApplicantUsername = applicantUsername.String
	}
	if consumerName.Valid {
		r.ConsumerName = consumerName.String
	}
	if desc.Valid {
		r.Description = desc.String
	}
	if reviewedBy.Valid {
		r.ReviewedBy = reviewedBy.String
	}
	if reviewedAt.Valid {
		r.ReviewedAt = reviewedAt.String
	}
	if createdAt.Valid {
		r.CreatedAt = createdAt.String
	}
	if updatedAt.Valid {
		r.UpdatedAt = updatedAt.String
	}
	return &r, nil
}

func (s *Store) UpdateAPIKeyRequest(id string, r *APIKeyRequest) (*APIKeyRequest, error) {
	intID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return nil, err
	}
	_, err = s.db.Exec(
		`UPDATE api_key_requests SET key_name=$2, consumer_name=$3, description=$4, status=$5,
		 reviewed_by=$6, reviewed_at=NOW(), updated_at=NOW() WHERE id=$1`,
		intID, r.KeyName, nullString(r.ConsumerName), nullString(r.Description), r.Status, nullString(r.ReviewedBy),
	)
	if err != nil {
		return nil, err
	}
	return s.GetAPIKeyRequest(id)
}

func (s *Store) DeleteAPIKeyRequest(id string) error {
	intID, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		return err
	}
	_, err = s.db.Exec(`DELETE FROM api_key_requests WHERE id=$1`, intID)
	return err
}

// ── Config Snapshots ───────────────────────────────────────────────────────

func (s *Store) ListConfigSnapshots() ([]ConfigSnapshot, error) {
	rows, err := s.db.Query(
		`SELECT id, version_label, diff_from_prev, actor_user_id, actor_username, created_at
		 FROM config_snapshots ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var snaps []ConfigSnapshot
	for rows.Next() {
		var sn ConfigSnapshot
		var diffFromPrev sql.NullString
		var actorUID, actorUname sql.NullString
		var createdAt sql.NullString
		if err := rows.Scan(&sn.ID, &sn.VersionLabel, &diffFromPrev, &actorUID, &actorUname, &createdAt); err != nil {
			return nil, err
		}
		if diffFromPrev.Valid {
			sn.DiffFromPrev = &diffFromPrev.String
		}
		if actorUID.Valid {
			sn.ActorUserID = actorUID.String
		}
		if actorUname.Valid {
			sn.ActorUsername = actorUname.String
		}
		if createdAt.Valid {
			sn.CreatedAt = createdAt.String
		}
		snaps = append(snaps, sn)
	}
	return snaps, nil
}

func (s *Store) CreateConfigSnapshot(sn *ConfigSnapshot) (*ConfigSnapshot, error) {
	var outID int64
	err := s.db.QueryRow(
		`INSERT INTO config_snapshots (version_label, diff_from_prev, actor_user_id, actor_username)
		 VALUES ($1,$2,$3,$4) RETURNING id, created_at`,
		sn.VersionLabel, nullString(stringVal(sn.DiffFromPrev)), nullString(sn.ActorUserID), nullString(sn.ActorUsername),
	).Scan(&outID, &sn.CreatedAt)
	if err != nil {
		return nil, err
	}
	sn.ID = outID
	return sn, nil
}

func (s *Store) GetConfigSnapshot(id string) (*ConfigSnapshot, error) {
	var sn ConfigSnapshot
	var diffFromPrev sql.NullString
	var actorUID, actorUname sql.NullString
	var createdAt sql.NullString
	err := s.db.QueryRow(
		`SELECT id, version_label, diff_from_prev, actor_user_id, actor_username, created_at
		 FROM config_snapshots WHERE id=$1`, id,
	).Scan(&sn.ID, &sn.VersionLabel, &diffFromPrev, &actorUID, &actorUname, &createdAt)
	if err != nil {
		return nil, err
	}
	if diffFromPrev.Valid {
		sn.DiffFromPrev = &diffFromPrev.String
	}
	if actorUID.Valid {
		sn.ActorUserID = actorUID.String
	}
	if actorUname.Valid {
		sn.ActorUsername = actorUname.String
	}
	if createdAt.Valid {
		sn.CreatedAt = createdAt.String
	}
	return &sn, nil
}

func (s *Store) DeleteConfigSnapshot(id string) error {
	_, err := s.db.Exec(`DELETE FROM config_snapshots WHERE id=$1`, id)
	return err
}

func (s *Store) SeedDefaultUsers() error {
	// Check if admin already exists
	existing, _ := s.GetUserByUsername("admin")
	if existing != nil {
		return nil
	}
	// Create admin with bcrypt hash of "admin123"
	hash, err := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
	if err != nil {
		return err
	}
	_, err = s.CreateUser(&User{
		Username:    "admin",
		PasswordHash: string(hash),
		DisplayName: "Administrator",
		Email:       "admin@cont.local",
		Role:        "admin",
	})
	if err != nil {
		return err
	}
	// Create regular user
	hash2, _ := bcrypt.GenerateFromPassword([]byte("user123"), bcrypt.DefaultCost)
	_, err = s.CreateUser(&User{
		Username:    "user",
		PasswordHash: string(hash2),
		DisplayName: "Regular User",
		Email:       "user@cont.local",
		Role:        "user",
	})
	return err
}

// GetOrCreateOAuthUser finds an existing user by OAuth provider+subject or creates a new one
func (s *Store) GetOrCreateOAuthUser(provider, subject, email, name string) (*User, error) {
	// Try to find existing user
	user, err := s.GetUserByOAuth(provider, subject)
	if err == nil && user != nil {
		return user, nil
	}

	// Create new user
	username := sanitizeOAuthUsername(email, provider)
	displayName := name
	if displayName == "" {
		displayName = username
	}

	newUser := &User{
		Username:    username,
		DisplayName: displayName,
		Email:       email,
		Role:        "viewer", // Default role for OAuth users
		Enabled:     true,
	}

	// Try to create; if email/username collision, fetch existing
	created, err := s.CreateUser(newUser)
	if err != nil {
		// Try to get by email as fallback
		userByEmail, err2 := s.GetUserByEmail(email)
		if err2 == nil && userByEmail != nil {
			// Link OAuth to existing user
			s.db.Exec(`UPDATE users SET oauth_provider=$1, oauth_subject=$2 WHERE id=$3`,
				provider, subject, userByEmail.ID)
			return userByEmail, nil
		}
		return nil, fmt.Errorf("user creation failed: %w", err)
	}

	// Link OAuth identity
	s.db.Exec(`UPDATE users SET oauth_provider=$1, oauth_subject=$2 WHERE id=$3`,
		provider, subject, created.ID)

	return created, nil
}

// GetUserByOAuth retrieves a user by OAuth provider and subject ID
func (s *Store) GetUserByOAuth(provider, subject string) (*User, error) {
	var user User
	var displayName, email sql.NullString
	err := s.db.QueryRow(`
		SELECT id, username, display_name, email, role, enabled, created_at, updated_at
		FROM users WHERE oauth_provider = $1 AND oauth_subject = $2`,
		provider, subject,
	).Scan(&user.ID, &user.Username, &displayName, &email,
		&user.Role, &user.Enabled, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, err
	}
	user.DisplayName = displayName.String
	user.Email = email.String
	groups, _ := s.GetUserGroups(user.ID)
	user.Groups = groups
	return &user, nil
}

// GetUserByEmail retrieves a user by email
func (s *Store) GetUserByEmail(email string) (*User, error) {
	var user User
	var displayName, emailStr sql.NullString
	err := s.db.QueryRow(`
		SELECT id, username, display_name, email, role, enabled, created_at, updated_at
		FROM users WHERE email = $1`, email,
	).Scan(&user.ID, &user.Username, &displayName, &emailStr,
		&user.Role, &user.Enabled, &user.CreatedAt, &user.UpdatedAt)
	if err != nil {
		return nil, err
	}
	user.DisplayName = displayName.String
	user.Email = emailStr.String
	groups, _ := s.GetUserGroups(user.ID)
	user.Groups = groups
	return &user, nil
}

func sanitizeOAuthUsername(email, provider string) string {
	if email != "" {
		atIdx := strings.Index(email, "@")
		if atIdx > 0 {
			return provider + "_" + email[:atIdx]
		}
	}
	b := make([]byte, 8)
	if _, err := rand.Read(b); err == nil {
		return provider + "_" + base64.URLEncoding.EncodeToString(b)
	}
	return provider + "_user"
}

// RecordFailedLogin records a failed login attempt for brute-force protection
func (s *Store) RecordFailedLogin(username, ipAddress string) error {
	_, err := s.db.Exec(
		`INSERT INTO login_attempts (username, ip_address, success) VALUES ($1, $2, false)`,
		username, ipAddress,
	)
	return err
}

// ClearFailedLogins clears all failed attempts for a user (on successful login)
func (s *Store) ClearFailedLogins(username string) error {
	_, err := s.db.Exec(`DELETE FROM login_attempts WHERE username=$1 AND success=false`, username)
	return err
}

// IsLockedOut returns true if the user has failed more than maxAttempts in the given window
func (s *Store) IsLockedOut(username string, maxAttempts int, windowSeconds int) (bool, error) {
	var count int
	err := s.db.QueryRow(`
		SELECT COUNT(*) FROM login_attempts
		WHERE username=$1 AND success=false
		AND attempted_at > NOW() - INTERVAL '1 second' * $2
	`, username, windowSeconds).Scan(&count)
	if err != nil {
		return false, err
	}
	return count >= maxAttempts, nil
}

// GetLoginAttemptsByIP returns the number of failed attempts from a given IP in the window
func (s *Store) GetLoginAttemptsByIP(ipAddress string, windowSeconds int) (int, error) {
	var count int
	err := s.db.QueryRow(`
		SELECT COUNT(*) FROM login_attempts
		WHERE ip_address=$1 AND success=false
		AND attempted_at > NOW() - INTERVAL '1 second' * $2
	`, ipAddress, windowSeconds).Scan(&count)
	if err != nil {
		return 0, err
	}
	return count, nil
}
