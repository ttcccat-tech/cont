package storage

import (
	"database/sql"
	"encoding/json"
	"strings"

	_ "github.com/lib/pq"
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
