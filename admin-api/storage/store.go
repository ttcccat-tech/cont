package storage

import (
	"crypto/rand"
	"database/sql"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"reflect"
	"strconv"
	"strings"
	"time"

	_ "github.com/lib/pq"
	"golang.org/x/crypto/bcrypt"
)

func NewStore(db *sql.DB, rdb *Redis) *Store {
	return &Store{db: db, rdb: rdb}
}

// ── Services ───────────────────────────────────────────────────────────────

func (s *Store) ListServices(orgID string, limit, offset int) ([]Service, error) {
	query := `
		SELECT id, name, protocol, host, port, path, url, retries,
		       connect_timeout, read_timeout, write_timeout, enabled,
		       COALESCE(org_id, '00000000-0000-0000-0000-000000000000') as org_id, created_at, updated_at
		FROM services
		WHERE (($1 = '' AND COALESCE(org_id::text, '00000000-0000-0000-0000-000000000000') = '00000000-0000-0000-0000-000000000000') OR ($1 != '' AND org_id::text = $1))
		ORDER BY created_at DESC LIMIT $2 OFFSET $3`
	rows, err := s.db.Query(query, orgID, limit, offset)
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
			&retries, &connect, &read, &write, &enabled, &r.OrgID, &created, &updated); err != nil {
			return nil, err
		}
		r.Name = name.String
		r.Protocol = protocol.String
		r.Host = host.String
		if port.Valid { r.Port = int(port.Int64) }
		r.Path = path.String
		r.URL = url.String
		if retries.Valid { r.Retries = int(retries.Int64) }
		if connect.Valid { r.ConnectTimeout = int(connect.Int64) }
		if read.Valid { r.ReadTimeout = int(read.Int64) }
		if write.Valid { r.WriteTimeout = int(write.Int64) }
		if enabled.Valid { r.Enabled = enabled.Bool }
		if created.Valid { r.CreatedAt = created.String }
		if updated.Valid { r.UpdatedAt = updated.String }
		out = append(out, r)
	}
	return out, nil
}

func (s *Store) CreateService(svc *Service) (*Service, error) {
	var id string
	orgID := svc.OrgID
	if orgID == "" {
		orgID = "00000000-0000-0000-0000-000000000000"
	}
	err := s.db.QueryRow(`
		INSERT INTO services (name, protocol, host, port, path, url, retries,
			connect_timeout, read_timeout, write_timeout, enabled, org_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12)
		RETURNING id, created_at, updated_at`,
		svc.Name, orString(svc.Protocol, "http"), svc.Host, orInt(svc.Port, 80),
		svc.Path, svc.URL, orInt(svc.Retries, 5),
		orInt(svc.ConnectTimeout, 60000), orInt(svc.ReadTimeout, 60000),
		orInt(svc.WriteTimeout, 60000), orBool(svc.Enabled, true), orgID,
	).Scan(&id, &svc.CreatedAt, &svc.UpdatedAt)
	if err != nil {
		return nil, err
	}
	svc.ID = id
	svc.OrgID = orgID
	// Auto-create a Resource entry for this service (for resource-level RBAC)
	s.ensureResourceEntry(id, svc.Name, "service")
	return svc, nil
}

func (s *Store) ensureResourceEntry(id, name, resourceType string) {
	if name == "" {
		name = id
	}
	_, _ = s.db.Exec(`
		INSERT INTO resources (id, name, path, type)
		VALUES ($1, $2, $3, $4)
		ON CONFLICT (id) DO UPDATE SET name = EXCLUDED.name, type = EXCLUDED.type`,
		id, name, "/"+resourceType+"/"+id, resourceType)
}

func (s *Store) GetService(id, orgID string) (*Service, error) {
	return s.getOneService(s.db.QueryRow(`
		SELECT id, name, protocol, host, port, path, url, retries,
		       connect_timeout, read_timeout, write_timeout, enabled,
		       COALESCE(org_id, '00000000-0000-0000-0000-000000000000') as org_id, created_at, updated_at
		FROM services WHERE id = $1 AND ((($2 = '' AND org_id IS NULL) OR ($2 = '' AND org_id = '00000000-0000-0000-0000-000000000000') OR COALESCE(org_id::text, '00000000-0000-0000-0000-000000000000') = $2))`, id, orgID))
}

func (s *Store) UpdateService(id, orgID string, svc *Service) (*Service, error) {
	err := s.db.QueryRow(`
		UPDATE services SET
			name=$2, protocol=$3, host=$4, port=$5, path=$6, url=$7,
			retries=$8, connect_timeout=$9, read_timeout=$10,
			write_timeout=$11, enabled=$12, updated_at=NOW()
		WHERE id=$1 AND ($13 = '' OR ($13 != '' AND org_id::text = $13)) RETURNING updated_at`,
		id, svc.Name, orString(svc.Protocol, "http"), svc.Host,
		orInt(svc.Port, 80), svc.Path, svc.URL, orInt(svc.Retries, 5),
		orInt(svc.ConnectTimeout, 60000), orInt(svc.ReadTimeout, 60000),
		orInt(svc.WriteTimeout, 60000), orBool(svc.Enabled, true),
		orgID,
	).Scan(&svc.UpdatedAt)
	if err != nil {
		return nil, err
	}
	svc.ID = id
	return svc, nil
}

func (s *Store) DeleteService(id, orgID string) error {
	_, err := s.db.Exec("DELETE FROM services WHERE id=$1 AND ((($2 = '' AND org_id IS NULL) OR ($2 = '' AND org_id = '00000000-0000-0000-0000-000000000000') OR COALESCE(org_id::text, '00000000-0000-0000-0000-000000000000') = $2))", id, orgID)
	return err
}

func (s *Store) getOneService(row *sql.Row) (*Service, error) {
	var r Service
	var name, protocol, host, path, url sql.NullString
	var port, retries, connect, read, write sql.NullInt64
	var enabled sql.NullBool
	var created, updated sql.NullString
	err := row.Scan(&r.ID, &name, &protocol, &host, &port, &path, &url,
		&retries, &connect, &read, &write, &enabled, &r.OrgID, &created, &updated)
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

func (s *Store) GetServiceByName(name, orgID string) (*Service, error) {
	row := s.db.QueryRow(`
		SELECT id, name, protocol, host, port, path, url, retries,
		       connect_timeout, read_timeout, write_timeout, enabled,
		       COALESCE(org_id, '00000000-0000-0000-0000-000000000000') as org_id, created_at, updated_at
		FROM services WHERE name=$1 AND ((($2 = '' AND org_id IS NULL) OR ($2 = '' AND org_id = '00000000-0000-0000-0000-000000000000') OR COALESCE(org_id::text, '00000000-0000-0000-0000-000000000000') = $2))`, name, orgID)
	return s.getOneService(row)
}

// ── gRPC Services ───────────────────────────────────────────────────────────

func (s *Store) ListGrpcServices(orgID string, limit, offset int) ([]GrpcService, error) {
	query := `
		SELECT id, name, package, proto_file, upstream_id, enabled,
		       COALESCE(org_id, '00000000-0000-0000-0000-000000000000') as org_id, created_at, updated_at
		FROM grpc_services
		WHERE (($1 = '' AND COALESCE(org_id::text, '00000000-0000-0000-0000-000000000000') = '00000000-0000-0000-0000-000000000000') OR ($1 != '' AND org_id::text = $1))
		ORDER BY created_at DESC LIMIT $2 OFFSET $3`
	rows, err := s.db.Query(query, orgID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []GrpcService
	for rows.Next() {
		var r GrpcService
		var name, pkg, proto, upstreamID sql.NullString
		var enabled sql.NullBool
		var created, updated sql.NullString
		if err := rows.Scan(&r.ID, &name, &pkg, &proto, &upstreamID, &enabled, &r.OrgID, &created, &updated); err != nil {
			return nil, err
		}
		r.Name = name.String
		r.Package = pkg.String
		r.ProtoFile = proto.String
		r.UpstreamID = upstreamID.String
		if enabled.Valid { r.Enabled = enabled.Bool }
		if created.Valid { r.CreatedAt = created.String }
		if updated.Valid { r.UpdatedAt = updated.String }
		out = append(out, r)
	}
	return out, nil
}

func (s *Store) CreateGrpcService(gs *GrpcService) (*GrpcService, error) {
	var id string
	orgID := gs.OrgID
	if orgID == "" {
		orgID = "00000000-0000-0000-0000-000000000000"
	}
	err := s.db.QueryRow(`
		INSERT INTO grpc_services (name, package, proto_file, upstream_id, enabled, org_id)
		VALUES ($1, $2, $3, NULLIF($4, '')::uuid, $5, $6)
		RETURNING id, created_at, updated_at`,
		gs.Name, orString(gs.Package, ""), orString(gs.ProtoFile, ""),
		gs.UpstreamID, orBool(gs.Enabled, true), orgID,
	).Scan(&id, &gs.CreatedAt, &gs.UpdatedAt)
	if err != nil {
		return nil, err
	}
	gs.ID = id
	gs.OrgID = orgID
	s.ensureResourceEntry(id, gs.Name, "grpc_service")
	return gs, nil
}

func (s *Store) GetGrpcService(id, orgID string) (*GrpcService, error) {
	row := s.db.QueryRow(`
		SELECT id, name, package, proto_file, upstream_id, enabled,
		       COALESCE(org_id, '00000000-0000-0000-0000-000000000000') as org_id, created_at, updated_at
		FROM grpc_services WHERE id = $1 AND (($2 = '' OR org_id::text = $2))`, id, orgID)
	var r GrpcService
	var name, pkg, proto, upstreamID sql.NullString
	var enabled sql.NullBool
	var created, updated sql.NullString
	err := row.Scan(&r.ID, &name, &pkg, &proto, &upstreamID, &enabled, &r.OrgID, &created, &updated)
	if err != nil {
		return nil, err
	}
	r.Name = name.String
	r.Package = pkg.String
	r.ProtoFile = proto.String
	r.UpstreamID = upstreamID.String
	if enabled.Valid { r.Enabled = enabled.Bool }
	if created.Valid { r.CreatedAt = created.String }
	if updated.Valid { r.UpdatedAt = updated.String }
	return &r, nil
}

func (s *Store) UpdateGrpcService(id, orgID string, gs *GrpcService) (*GrpcService, error) {
	row := s.db.QueryRow(`
		UPDATE grpc_services SET
			name=$2, package=$3, proto_file=$4, upstream_id=NULLIF($5, '')::uuid, enabled=$6, updated_at=NOW()
		WHERE id=$1 AND ($7 = '' OR org_id::text = $7)
		RETURNING id, name, package, proto_file, upstream_id, enabled,
		          COALESCE(org_id, '00000000-0000-0000-0000-000000000000') as org_id, updated_at`,
		id, gs.Name, orString(gs.Package, ""), orString(gs.ProtoFile, ""),
		gs.UpstreamID, orBool(gs.Enabled, true), orgID,
	)
	var r GrpcService
	var name, pkg, proto, upstreamID sql.NullString
	var enabled sql.NullBool
	var updated sql.NullString
	err := row.Scan(&r.ID, &name, &pkg, &proto, &upstreamID, &enabled, &r.OrgID, &updated)
	if err != nil {
		return nil, err
	}
	r.Name = name.String
	r.Package = pkg.String
	r.ProtoFile = proto.String
	r.UpstreamID = upstreamID.String
	if enabled.Valid { r.Enabled = enabled.Bool }
	if updated.Valid { r.UpdatedAt = updated.String }
	return &r, nil
}

func (s *Store) DeleteGrpcService(id, orgID string) error {
	_, err := s.db.Exec("DELETE FROM grpc_services WHERE id=$1 AND ($2 = '' OR org_id::text = $2)", id, orgID)
	return err
}

// ── gRPC Methods ────────────────────────────────────────────────────────────

func (s *Store) ListGrpcMethods(serviceID, orgID string) ([]GrpcMethod, error) {
	query := `
		SELECT id, service_id, name, method_type, input_type, output_type, enabled,
		       COALESCE(org_id, '00000000-0000-0000-0000-000000000000') as org_id, created_at, updated_at
		FROM grpc_methods
		WHERE service_id = $1::uuid AND ($2 = '' OR org_id::text = $2)
		ORDER BY created_at ASC`
	rows, err := s.db.Query(query, serviceID, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []GrpcMethod
	for rows.Next() {
		var r GrpcMethod
		var name, methodType, inputType, outputType sql.NullString
		var enabled sql.NullBool
		var created, updated sql.NullString
		if err := rows.Scan(&r.ID, &r.ServiceID, &name, &methodType, &inputType, &outputType, &enabled, &r.OrgID, &created, &updated); err != nil {
			return nil, err
		}
		r.Name = name.String
		r.MethodType = methodType.String
		r.InputType = inputType.String
		r.OutputType = outputType.String
		if enabled.Valid { r.Enabled = enabled.Bool }
		if created.Valid { r.CreatedAt = created.String }
		if updated.Valid { r.UpdatedAt = updated.String }
		out = append(out, r)
	}
	return out, nil
}

func (s *Store) CreateGrpcMethod(gm *GrpcMethod) (*GrpcMethod, error) {
	var id string
	orgID := gm.OrgID
	if orgID == "" {
		orgID = "00000000-0000-0000-0000-000000000000"
	}
	err := s.db.QueryRow(`
		INSERT INTO grpc_methods (service_id, name, method_type, input_type, output_type, enabled, org_id)
		VALUES ($1::uuid, $2, $3, $4, $5, $6, $7)
		RETURNING id, created_at, updated_at`,
		gm.ServiceID, gm.Name, orString(gm.MethodType, "unary"),
		orString(gm.InputType, ""), orString(gm.OutputType, ""),
		orBool(gm.Enabled, true), orgID,
	).Scan(&id, &gm.CreatedAt, &gm.UpdatedAt)
	if err != nil {
		return nil, err
	}
	gm.ID = id
	gm.OrgID = orgID
	return gm, nil
}

func (s *Store) DeleteGrpcMethod(id, orgID string) error {
	_, err := s.db.Exec("DELETE FROM grpc_methods WHERE id=$1 AND ($2 = '' OR org_id::text = $2)", id, orgID)
	return err
}

// ── Routes ─────────────────────────────────────────────────────────────────

func (s *Store) ListRoutes(orgID string, limit, offset int) ([]Route, error) {
	query := `
		SELECT id, name, service_id, protocols, hosts, paths, methods,
		       strip_path, preserve_host, regex_priority,
		       https_redirect_status_code, connection_timeout, enabled,
		       COALESCE(org_id, '00000000-0000-0000-0000-000000000000') as org_id, created_at, updated_at
		FROM routes
		WHERE (($1 = '' AND COALESCE(org_id::text, '00000000-0000-0000-0000-000000000000') = '00000000-0000-0000-0000-000000000000') OR ($1 != '' AND org_id::text = $1))
		ORDER BY created_at DESC LIMIT $2 OFFSET $3`
	rows, err := s.db.Query(query, orgID, limit, offset)
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
			&httpsStatus, &connTimeout, &enabled, &r.OrgID, &created, &updated); err != nil {
			return nil, err
		}
		jsonScanSlice(&r.Protocols, protocols)
		jsonScanSlice(&r.Hosts, hosts)
		jsonScanSlice(&r.Paths, paths)
		jsonScanSlice(&r.Methods, methods)
		if name.Valid { r.Name = name.String }
		if serviceID.Valid { r.Service = &ServiceRef{ID: serviceID.String} }
		if stripPath.Valid { r.StripPath = stripPath.Bool }
		if preserveHost.Valid { r.PreserveHost = preserveHost.Bool }
		if regexPriority.Valid { r.RegexPriority = int(regexPriority.Int64) }
		if httpsStatus.Valid { r.HTTPSRedirectStatusCode = int(httpsStatus.Int64) }
		if connTimeout.Valid { r.ConnectionTimeout = int(connTimeout.Int64) }
		if enabled.Valid { r.Enabled = enabled.Bool }
		if created.Valid { r.CreatedAt = created.String }
		if updated.Valid { r.UpdatedAt = updated.String }
		out = append(out, r)
	}
	return out, nil
}

func (s *Store) CreateRoute(r *Route) (*Route, error) {
	protocols := orSlice(r.Protocols, []string{"http", "https"})
	hosts := orSlice(r.Hosts, []string{})
	paths := orSlice(r.Paths, []string{})
	methods := orSlice(r.Methods, []string{})
	orgID := r.OrgID
	if orgID == "" {
		orgID = "00000000-0000-0000-0000-000000000000"
	}
	err := s.db.QueryRow(`
		INSERT INTO routes (name, service_id, protocols, hosts, paths, methods,
			strip_path, preserve_host, regex_priority, https_redirect_status_code,
			connection_timeout, enabled, org_id)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13)
		RETURNING id, created_at, updated_at`,
		r.Name, r.GetServiceID(), "{"+strings.Join(protocols,",")+"}", "{"+strings.Join(hosts,",")+"}", "{"+strings.Join(paths,",")+"}", "{"+strings.Join(methods,",")+"}",
		orBool(r.StripPath, true), orBool(r.PreserveHost, false),
		orInt(r.RegexPriority, 0), orInt(r.HTTPSRedirectStatusCode, 426),
		orInt(r.ConnectionTimeout, 60000), orBool(r.Enabled, true), orgID,
	).Scan(&r.ID, &r.CreatedAt, &r.UpdatedAt)
	if err != nil {
		return nil, err
	}
	r.OrgID = orgID
	// Auto-create a Resource entry for this route (for resource-level RBAC)
	s.ensureResourceEntry(r.ID, r.Name, "route")
	return r, nil
}

func (s *Store) GetRoute(id, orgID string) (*Route, error) {
	row := s.db.QueryRow(`
		SELECT id, name, service_id, protocols, hosts, paths, methods,
		       strip_path, preserve_host, regex_priority,
		       https_redirect_status_code, connection_timeout, enabled,
		       COALESCE(org_id, '00000000-0000-0000-0000-000000000000') as org_id, created_at, updated_at
		FROM routes WHERE id = $1 AND ((($2 = '' AND org_id IS NULL) OR ($2 = '' AND org_id = '00000000-0000-0000-0000-000000000000') OR COALESCE(org_id::text, '00000000-0000-0000-0000-000000000000') = $2))`, id, orgID)
	var r Route
	var name, serviceID sql.NullString
	var protocols, hosts, paths, methods []byte
	var stripPath, preserveHost sql.NullBool
	var regexPriority, httpsStatus, connTimeout sql.NullInt64
	var enabled sql.NullBool
	var created, updated sql.NullString
	err := row.Scan(&r.ID, &name, &serviceID, &protocols, &hosts,
		&paths, &methods, &stripPath, &preserveHost, &regexPriority,
		&httpsStatus, &connTimeout, &enabled, &r.OrgID, &created, &updated)
	if err != nil {
		return nil, err
	}
	if name.Valid { r.Name = name.String }
	if serviceID.Valid { r.Service = &ServiceRef{ID: serviceID.String} }
	jsonScanSlice(&r.Protocols, protocols)
	jsonScanSlice(&r.Hosts, hosts)
	jsonScanSlice(&r.Paths, paths)
	jsonScanSlice(&r.Methods, methods)
	if stripPath.Valid { r.StripPath = stripPath.Bool }
	if preserveHost.Valid { r.PreserveHost = preserveHost.Bool }
	if regexPriority.Valid { r.RegexPriority = int(regexPriority.Int64) }
	if httpsStatus.Valid { r.HTTPSRedirectStatusCode = int(httpsStatus.Int64) }
	if connTimeout.Valid { r.ConnectionTimeout = int(connTimeout.Int64) }
	if enabled.Valid { r.Enabled = enabled.Bool }
	if created.Valid { r.CreatedAt = created.String }
	if updated.Valid { r.UpdatedAt = updated.String }
	return &r, nil
}

func (s *Store) UpdateRoute(id, orgID string, r *Route) (*Route, error) {
	protocols := orSlice(r.Protocols, []string{"http", "https"})
	hosts := orSlice(r.Hosts, []string{})
	paths := orSlice(r.Paths, []string{})
	methods := orSlice(r.Methods, []string{})

	setClauses := []string{"name=$2", "protocols=$3", "hosts=$4", "paths=$5", "methods=$6", "strip_path=$7", "preserve_host=$8", "regex_priority=$9", "https_redirect_status_code=$10", "connection_timeout=$11", "enabled=$12"}
	args := []interface{}{id, r.Name, "{"+strings.Join(protocols, ",")+"}", "{"+strings.Join(hosts, ",")+"}", "{"+strings.Join(paths, ",")+"}", "{"+strings.Join(methods, ",")+"}", orBool(r.StripPath, true), orBool(r.PreserveHost, false), orInt(r.RegexPriority, 0), orInt(r.HTTPSRedirectStatusCode, 426), orInt(r.ConnectionTimeout, 60000), orBool(r.Enabled, true)}
	if svcID := r.GetServiceID(); svcID != "" {
		setClauses = append([]string{"service_id=$13"}, setClauses...)
		args = append([]interface{}{svcID}, args...)
	}
	args = append(args, orgID)
	query := "UPDATE routes SET " + strings.Join(setClauses, ", ") + " WHERE id=$1 AND ($14 = '' OR ($14 != '' AND org_id::text = $14)) RETURNING updated_at"

	err := s.db.QueryRow(query, args...).Scan(&r.UpdatedAt)
	if err != nil {
		return nil, err
	}
	r.ID = id
	return r, nil
}

func (s *Store) DeleteRoute(id, orgID string) error {
	_, err := s.db.Exec("DELETE FROM routes WHERE id=$1 AND ((($2 = '' AND org_id IS NULL) OR ($2 = '' AND org_id = '00000000-0000-0000-0000-000000000000') OR COALESCE(org_id::text, '00000000-0000-0000-0000-000000000000') = $2))", id, orgID)
	return err
}

// ── Upstreams ──────────────────────────────────────────────────────────────

func (s *Store) ListUpstreams(orgID string, limit, offset int) ([]Upstream, error) {
	query := `
		SELECT id, name, algorithm, slots, healthchecks, enabled,
		       COALESCE(org_id, '00000000-0000-0000-0000-000000000000') as org_id, created_at, updated_at
		FROM upstreams
		WHERE (($1 = '' AND COALESCE(org_id::text, '00000000-0000-0000-0000-000000000000') = '00000000-0000-0000-0000-000000000000') OR ($1 != '' AND org_id::text = $1))
		ORDER BY created_at DESC LIMIT $2 OFFSET $3`
	rows, err := s.db.Query(query, orgID, limit, offset)
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
			&healthchecks, &enabled, &u.OrgID, &created, &updated); err != nil {
			return nil, err
		}
		u.Name = name.String
		u.Algorithm = orString(algorithm.String, "roundrobin")
		if slots.Valid { u.Slots = int(slots.Int64) }
		u.Healthchecks = healthchecks.String
		if enabled.Valid { u.Enabled = enabled.Bool }
		if created.Valid { u.CreatedAt = created.String }
		if updated.Valid { u.UpdatedAt = updated.String }
		out = append(out, u)
	}
	return out, nil
}

func (s *Store) CreateUpstream(u *Upstream) (*Upstream, error) {
	orgID := u.OrgID
	var orgIDArg interface{}
	if orgID != "" {
		orgIDArg = orgID
	} else {
		orgIDArg = nil
	}
	err := s.db.QueryRow(`
		INSERT INTO upstreams (name, algorithm, slots, healthchecks, enabled, org_id)
		VALUES ($1,$2,$3,$4,$5,$6) RETURNING id, created_at, updated_at`,
		u.Name, orString(u.Algorithm, "roundrobin"), orInt(u.Slots, 10000),
		u.Healthchecks, orBool(u.Enabled, true), orgIDArg,
	).Scan(&u.ID, &u.CreatedAt, &u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if orgID != "" {
		u.OrgID = orgID
	}
	// Auto-create a Resource entry for this upstream (for resource-level RBAC)
	s.ensureResourceEntry(u.ID, u.Name, "upstream")
	return u, nil
}

func (s *Store) GetUpstream(id, orgID string) (*Upstream, error) {
	var u Upstream
	var name, algorithm, healthchecks sql.NullString
	var slots sql.NullInt64
	var enabled sql.NullBool
	var created, updated sql.NullString
	err := s.db.QueryRow(`
		SELECT id, name, algorithm, slots, healthchecks, enabled,
		       COALESCE(org_id, '00000000-0000-0000-0000-000000000000') as org_id, created_at, updated_at
		FROM upstreams WHERE id=$1 AND ((($2 = '' AND org_id IS NULL) OR ($2 = '' AND org_id = '00000000-0000-0000-0000-000000000000') OR COALESCE(org_id::text, '00000000-0000-0000-0000-000000000000') = $2))`,
		id, orgID).Scan(
		&u.ID, &name, &algorithm, &slots, &healthchecks, &enabled, &u.OrgID, &created, &updated)
	if err != nil {
		return nil, err
	}
	u.Name = name.String
	u.Algorithm = algorithm.String
	if slots.Valid { u.Slots = int(slots.Int64) }
	u.Healthchecks = healthchecks.String
	if enabled.Valid { u.Enabled = enabled.Bool }
	if created.Valid { u.CreatedAt = created.String }
	if updated.Valid { u.UpdatedAt = updated.String }
	return &u, nil
}

func (s *Store) UpdateUpstream(id, orgID string, u *Upstream) (*Upstream, error) {
	err := s.db.QueryRow(`
		UPDATE upstreams SET name=$2, algorithm=COALESCE(NULLIF($3,''),'roundrobin'), slots=$4,
			healthchecks=$5, enabled=$6, updated_at=NOW()
		WHERE id=$1 AND ($7 = '' OR org_id::text = $7) RETURNING updated_at`,
		id, u.Name, u.Algorithm,
		orInt(u.Slots, 10000), u.Healthchecks, orBool(u.Enabled, true),
		orgID,
	).Scan(&u.UpdatedAt)
	if err != nil {
		return nil, err
	}
	u.ID = id
	return u, nil
}

func (s *Store) DeleteUpstream(id, orgID string) error {
	_, err := s.db.Exec("DELETE FROM upstreams WHERE id=$1 AND ((($2 = '' AND org_id IS NULL) OR ($2 = '' AND org_id = '00000000-0000-0000-0000-000000000000') OR COALESCE(org_id::text, '00000000-0000-0000-0000-000000000000') = $2))", id, orgID)
	return err
}

// ── Targets ─────────────────────────────────────────────────────────────────

func (s *Store) ListTargetsByUpstream(upstreamID string) ([]Target, error) {
	rows, err := s.db.Query(`
		SELECT id, target, weight, enabled,
		       COALESCE(org_id, '00000000-0000-0000-0000-000000000000') as org_id, created_at
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
		if err := rows.Scan(&t.ID, &target, &weight, &enabled, &t.OrgID, &created); err != nil {
			return nil, err
		}
		t.UpstreamID = upstreamID
		t.Target = target.String
		if weight.Valid { t.Weight = int(weight.Int64) }
		if enabled.Valid { t.Enabled = enabled.Bool }
		if created.Valid { t.CreatedAt = created.String }
		out = append(out, t)
	}
	return out, nil
}

func (s *Store) CreateTarget(t *Target) (*Target, error) {
	orgID := t.OrgID
	var orgIDArg interface{}
	if orgID != "" {
		orgIDArg = orgID
	} else {
		orgIDArg = nil
	}
	err := s.db.QueryRow(`
		INSERT INTO targets (upstream_id, target, weight, enabled, org_id)
		VALUES ($1,$2,$3,$4,$5) RETURNING id, created_at`,
		t.UpstreamID, t.Target, orInt(t.Weight, 100), orBool(t.Enabled, true), orgIDArg,
	).Scan(&t.ID, &t.CreatedAt)
	if err != nil {
		return nil, err
	}
	if orgID != "" {
		t.OrgID = orgID
	}
	return t, nil
}

func (s *Store) UpdateTarget(upstreamID, targetID, orgID string, t *Target) (*Target, error) {
	err := s.db.QueryRow(`
		UPDATE targets SET target=$2, weight=$3, enabled=$4
		WHERE id=$1 AND upstream_id=$5 AND (($6 = '' AND org_id IS NULL) OR org_id::text = $6) RETURNING id`,
		targetID, t.Target, orInt(t.Weight, 100), orBool(t.Enabled, true), upstreamID, orgID,
	).Scan(&t.ID)
	if err != nil {
		return nil, err
	}
	t.UpstreamID = upstreamID
	return t, nil
}

func (s *Store) DeleteTarget(upstreamID, targetID, orgID string) error {
	_, err := s.db.Exec("DELETE FROM targets WHERE id=$1 AND upstream_id=$2 AND (($3 = '' AND org_id IS NULL) OR org_id::text = $3)", targetID, upstreamID, orgID)
	return err
}

// ── Consumers ───────────────────────────────────────────────────────────────

func (s *Store) ListConsumers(orgID string, limit, offset int) ([]Consumer, error) {
	query := `
		SELECT id, username, custom_id, enabled,
		       COALESCE(org_id, '00000000-0000-0000-0000-000000000000') as org_id, created_at, updated_at
		FROM consumers
		WHERE (($1 = '' AND COALESCE(org_id::text, '00000000-0000-0000-0000-000000000000') = '00000000-0000-0000-0000-000000000000') OR ($1 != '' AND org_id::text = $1))
		ORDER BY created_at DESC LIMIT $2 OFFSET $3`
	rows, err := s.db.Query(query, orgID, limit, offset)
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
		if err := rows.Scan(&c.ID, &username, &customID, &enabled, &c.OrgID, &created, &updated); err != nil {
			return nil, err
		}
		c.Username = username.String
		c.CustomID = customID.String
		if enabled.Valid { c.Enabled = enabled.Bool }
		if created.Valid { c.CreatedAt = created.String }
		if updated.Valid { c.UpdatedAt = updated.String }
		out = append(out, c)
	}
	return out, nil
}

func (s *Store) CreateConsumer(c *Consumer) (*Consumer, error) {
	orgID := c.OrgID
	var orgIDArg interface{}
	if orgID != "" {
		orgIDArg = orgID
	} else {
		orgIDArg = nil
	}
	err := s.db.QueryRow(`
		INSERT INTO consumers (username, custom_id, enabled, org_id)
		VALUES ($1,$2,$3,$4) RETURNING id, created_at, updated_at`,
		c.Username, c.CustomID, orBool(c.Enabled, true), orgIDArg,
	).Scan(&c.ID, &c.CreatedAt, &c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if orgID != "" {
		c.OrgID = orgID
	}
	// Auto-create a Resource entry for this consumer (for resource-level RBAC)
	s.ensureResourceEntry(c.ID, c.Username, "consumer")
	return c, nil
}

func (s *Store) GetConsumer(id, orgID string) (*Consumer, error) {
	var c Consumer
	var username, customID sql.NullString
	var enabled sql.NullBool
	var created, updated sql.NullString
	err := s.db.QueryRow(`
		SELECT id, username, custom_id, enabled,
		       COALESCE(org_id, '00000000-0000-0000-0000-000000000000') as org_id, created_at, updated_at
		FROM consumers WHERE id=$1 AND ((($2 = '' AND org_id IS NULL) OR ($2 = '' AND org_id = '00000000-0000-0000-0000-000000000000') OR COALESCE(org_id::text, '00000000-0000-0000-0000-000000000000') = $2))`,
		id, orgID).Scan(
		&c.ID, &username, &customID, &enabled, &c.OrgID, &created, &updated)
	if err != nil {
		return nil, err
	}
	c.Username = username.String
	c.CustomID = customID.String
	if enabled.Valid { c.Enabled = enabled.Bool }
	if created.Valid { c.CreatedAt = created.String }
	if updated.Valid { c.UpdatedAt = updated.String }
	return &c, nil
}

func (s *Store) UpdateConsumer(id, orgID string, c *Consumer) (*Consumer, error) {
	err := s.db.QueryRow(`
		UPDATE consumers SET username=$2, custom_id=$3, enabled=$4, updated_at=NOW()
		WHERE id=$1 AND ($5 = '' OR org_id::text = $5) RETURNING updated_at`,
		id, c.Username, c.CustomID, orBool(c.Enabled, true),
		orgID,
	).Scan(&c.UpdatedAt)
	if err != nil {
		return nil, err
	}
	c.ID = id
	return c, nil
}

func (s *Store) DeleteConsumer(id, orgID string) error {
	_, err := s.db.Exec("DELETE FROM consumers WHERE id=$1 AND ((($2 = '' AND org_id IS NULL) OR ($2 = '' AND org_id = '00000000-0000-0000-0000-000000000000') OR COALESCE(org_id::text, '00000000-0000-0000-0000-000000000000') = $2))", id, orgID)
	return err
}

// ── Consumer Credentials ───────────────────────────────────────────────────

// ListConsumerCredentials returns all credentials for a consumer of a given type
func (s *Store) ListConsumerCredentials(consumerID, credentialType string) ([]ConsumerCredential, error) {
	rows, err := s.db.Query(`
		SELECT id, consumer_id, credential_type, key, secret, enabled, expires_at, created_at
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
		var secret, expires, created sql.NullString
		if err := rows.Scan(&c.ID, &c.ConsumerID, &c.CredentialType, &c.Key, &secret, &c.Enabled, &expires, &created); err != nil {
			return nil, err
		}
		if secret.Valid {
			c.Secret = secret.String
		}
		if expires.Valid {
			c.ExpiresAt = &expires.String
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
		INSERT INTO consumer_credentials (consumer_id, credential_type, key, secret, enabled, expires_at)
		VALUES ($1, $2, $3, $4, $5, $6)
		RETURNING id, created_at`,
		c.ConsumerID, c.CredentialType, c.Key, c.Secret, orBool(c.Enabled, true), c.ExpiresAt,
	).Scan(&c.ID, &c.CreatedAt)
	if err != nil {
		return nil, err
	}
	return c, nil
}

// GetConsumerCredentialByKey looks up a credential by type and key (for auth middleware)
func (s *Store) GetConsumerCredentialByKey(credentialType, key string) (*ConsumerCredential, error) {
	var c ConsumerCredential
	var secret, expires, created sql.NullString
	err := s.db.QueryRow(`
		SELECT id, consumer_id, credential_type, key, secret, enabled, expires_at, created_at
		FROM consumer_credentials
		WHERE credential_type=$1 AND key=$2 AND enabled=true`,
		credentialType, key,
	).Scan(&c.ID, &c.ConsumerID, &c.CredentialType, &c.Key, &secret, &c.Enabled, &expires, &created)
	if err != nil {
		return nil, err
	}
	if secret.Valid {
		c.Secret = secret.String
	}
	if expires.Valid {
		c.ExpiresAt = &expires.String
	}
	if created.Valid {
		c.CreatedAt = created.String
	}
	return &c, nil
}

// GetConsumerCredential fetches a credential by ID for a consumer
func (s *Store) GetConsumerCredential(consumerID, credentialType, credentialID string) (*ConsumerCredential, error) {
	var c ConsumerCredential
	var secret, expires, created sql.NullString
	err := s.db.QueryRow(`
		SELECT id, consumer_id, credential_type, key, secret, enabled, expires_at, created_at
		FROM consumer_credentials
		WHERE id=$1 AND consumer_id=$2 AND credential_type=$3`,
		credentialID, consumerID, credentialType,
	).Scan(&c.ID, &c.ConsumerID, &c.CredentialType, &c.Key, &secret, &c.Enabled, &expires, &created)
	if err != nil {
		return nil, err
	}
	if secret.Valid {
		c.Secret = secret.String
	}
	if expires.Valid {
		c.ExpiresAt = &expires.String
	}
	if created.Valid {
		c.CreatedAt = created.String
	}
	return &c, nil
}

// UpdateConsumerCredential updates a credential's enabled and expires_at fields
func (s *Store) UpdateConsumerCredential(consumerID, credentialType, credentialID string, enabled *bool, expiresAt *string) error {
	query := `UPDATE consumer_credentials SET `
	args := []interface{}{}
	argIdx := 1
	if enabled != nil {
		query += fmt.Sprintf("enabled=$%d, ", argIdx)
		args = append(args, *enabled)
		argIdx++
	}
	if expiresAt != nil {
		query += fmt.Sprintf("expires_at=$%d, ", argIdx)
		if *expiresAt == "" {
			// Empty string means clear the expiry (set to NULL)
			args = append(args, nil)
		} else {
			args = append(args, *expiresAt)
		}
		argIdx++
	}
	// Nothing to update
	if argIdx == 1 {
		return nil
	}
	// Remove trailing comma
	query = query[:len(query)-2]
	query += fmt.Sprintf(" WHERE id=$%d AND consumer_id=$%d AND credential_type=$%d", argIdx, argIdx+1, argIdx+2)
	args = append(args, credentialID, consumerID, credentialType)

	result, err := s.db.Exec(query, args...)
	if err != nil {
		return err
	}
	affected, _ := result.RowsAffected()
	if affected == 0 {
		return sql.ErrNoRows
	}
	return nil
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

func (s *Store) ListPlugins(orgID string, limit, offset int) ([]Plugin, error) {
	query := `
		SELECT id, name, route_id, service_id, consumer_id, config, enabled,
		       COALESCE(org_id, '00000000-0000-0000-0000-000000000000') as org_id, scope, created_at, updated_at
		FROM plugins
		WHERE (($1 = '' AND COALESCE(org_id::text, '00000000-0000-0000-0000-000000000000') = '00000000-0000-0000-0000-000000000000') OR ($1 != '' AND org_id::text = $1))
		ORDER BY created_at DESC LIMIT $2 OFFSET $3`
	rows, err := s.db.Query(query, orgID, limit, offset)
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
		var scope sql.NullString
		var created, updated sql.NullString
		if err := rows.Scan(&p.ID, &name, &routeID, &serviceID, &consumerID,
			&config, &enabled, &p.OrgID, &scope, &created, &updated); err != nil {
			return nil, err
		}
		p.Name = name.String
		if routeID.Valid {
			p.Route = &PluginScope{ID: routeID.String}
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
		if scope.Valid {
			p.Scope = scope.String
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
	orgID := p.OrgID
	var orgIDArg interface{}
	if orgID != "" {
		orgIDArg = orgID
	} else {
		orgIDArg = nil
	}
	configJSON, _ := json.Marshal(p.Config)
	var routeID, serviceID, consumerID *string
	if p.Route != nil { routeID = &p.Route.ID }
	if p.Service != nil { serviceID = &p.Service.ID }
	if p.Consumer != nil { consumerID = &p.Consumer.ID }
	// Default scope to 'service' if not specified
	scope := p.Scope
	if scope == "" {
		scope = "service"
	}
	err := s.db.QueryRow(`
		INSERT INTO plugins (name, route_id, service_id, consumer_id, config, enabled, org_id, scope)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id, created_at, updated_at`,
		p.Name, routeID, serviceID, consumerID, configJSON, orBool(p.Enabled, true), orgIDArg, scope,
	).Scan(&p.ID, &p.CreatedAt, &p.UpdatedAt)
	if err != nil {
		return nil, err
	}
	if orgID != "" {
		p.OrgID = orgID
	}
	p.Scope = scope
	return p, nil
}

func (s *Store) GetPlugin(id, orgID string) (*Plugin, error) {
	var p Plugin
	var name sql.NullString
	var routeID, serviceID, consumerID sql.NullString
	var config []byte
	var enabled sql.NullBool
	var scope sql.NullString
	var created, updated sql.NullString
	err := s.db.QueryRow(`
		SELECT id, name, route_id, service_id, consumer_id, config, enabled,
		       COALESCE(org_id, '00000000-0000-0000-0000-000000000000') as org_id, scope, created_at, updated_at
		FROM plugins WHERE id=$1 AND ((($2 = '' AND org_id IS NULL) OR ($2 = '' AND org_id = '00000000-0000-0000-0000-000000000000') OR COALESCE(org_id::text, '00000000-0000-0000-0000-000000000000') = $2))`,
		id, orgID).Scan(
		&p.ID, &name, &routeID, &serviceID, &consumerID,
		&config, &enabled, &p.OrgID, &scope, &created, &updated)
	if err != nil {
		return nil, err
	}
	p.Name = name.String
	if routeID.Valid {
		p.Route = &PluginScope{ID: routeID.String}
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
	if scope.Valid {
		p.Scope = scope.String
	}
	if created.Valid {
		p.CreatedAt = created.String
	}
	if updated.Valid {
		p.UpdatedAt = updated.String
	}
	return &p, nil
}

func (s *Store) UpdatePlugin(id, orgID string, p *Plugin) (*Plugin, error) {
	// Fetch existing plugin so we only update fields that were actually provided
	existing, err := s.GetPlugin(id, orgID)
	if err != nil {
		return nil, err
	}
	// Merge: use p's non-zero/non-empty fields, fall back to existing
	name := p.Name
	if name == "" {
		name = existing.Name
	}
	configJSON, _ := json.Marshal(p.Config)
	if p.Config == nil {
		configJSON, _ = json.Marshal(existing.Config)
	}
	var routeID, serviceID, consumerID *string
	if p.Route != nil {
		routeID = &p.Route.ID
	} else if existing.Route != nil {
		routeID = &existing.Route.ID
	}
	if p.Service != nil {
		serviceID = &p.Service.ID
	} else if existing.Service != nil {
		serviceID = &existing.Service.ID
	}
	if p.Consumer != nil {
		consumerID = &p.Consumer.ID
	} else if existing.Consumer != nil {
		consumerID = &existing.Consumer.ID
	}
	enabled := p.Enabled
	if !p.Enabled && p.Enabled == existing.Enabled {
		enabled = existing.Enabled
	}
	scope := p.Scope
	if scope == "" {
		scope = existing.Scope
	}
	updatedAt, err := s.updatePluginFields(id, orgID, name, routeID, serviceID, consumerID, configJSON, enabled, scope)
	if err != nil {
		return nil, err
	}
	return &Plugin{
		ID: id, Name: name,
		Route: existing.Route, Service: existing.Service, Consumer: existing.Consumer,
		Config: p.Config, Enabled: enabled, Scope: scope,
		UpdatedAt: updatedAt,
	}, nil
}

func (s *Store) updatePluginFields(id, orgID, name string, routeID, serviceID, consumerID *string, configJSON []byte, enabled bool, scope string) (string, error) {
	var updatedAt string
	err := s.db.QueryRow(`
		UPDATE plugins SET name=$2, route_id=$3, service_id=$4, consumer_id=$5,
			config=$6, enabled=$7, scope=$8, updated_at=NOW()
		WHERE id=$1 AND (($9 = '' AND org_id IS NULL) OR org_id::text = $9) RETURNING updated_at`,
		id, name, routeID, serviceID, consumerID, configJSON, enabled, scope, orgID,
	).Scan(&updatedAt)
	return updatedAt, err
}

func (s *Store) DeletePlugin(id, orgID string) error {
	_, err := s.db.Exec("DELETE FROM plugins WHERE id=$1 AND ((($2 = '' AND org_id IS NULL) OR ($2 = '' AND org_id = '00000000-0000-0000-0000-000000000000') OR COALESCE(org_id::text, '00000000-0000-0000-0000-000000000000') = $2))", id, orgID)
	return err
}

// ── Workspaces ─────────────────────────────────────────────────────────────

func (s *Store) ListWorkspaces(orgID string) ([]Workspace, error) {
	query := `
		SELECT id, name, COALESCE(org_id, '00000000-0000-0000-0000-000000000000') as org_id, created_at
		FROM workspaces
		WHERE (($1 = '' AND COALESCE(org_id::text, '00000000-0000-0000-0000-000000000000') = '00000000-0000-0000-0000-000000000000') OR ($1 != '' AND org_id::text = $1))
		ORDER BY created_at DESC`
	rows, err := s.db.Query(query, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Workspace
	for rows.Next() {
		var w Workspace
		var name sql.NullString
		var created sql.NullString
		if err := rows.Scan(&w.ID, &name, &w.OrgID, &created); err != nil {
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
	orgID := w.OrgID
	if orgID == "" {
		orgID = "00000000-0000-0000-0000-000000000000"
	}
	err := s.db.QueryRow(`
		INSERT INTO workspaces (name, org_id) VALUES ($1, $2) RETURNING id, org_id, created_at`,
		w.Name, orgID).Scan(&w.ID, &w.OrgID, &w.CreatedAt)
	if err != nil {
		return nil, err
	}
	return w, nil
}

func (s *Store) GetWorkspace(id string, orgID string) (*Workspace, error) {
	var w Workspace
	var name sql.NullString
	var created sql.NullString
	query := `SELECT id, name, COALESCE(org_id, '00000000-0000-0000-0000-000000000000') as org_id, created_at FROM workspaces WHERE id=$1`
	args := []interface{}{id}
	if orgID != "" {
		query += ` AND ((($2 = '' AND org_id IS NULL) OR ($2 = '' AND org_id = '00000000-0000-0000-0000-000000000000') OR COALESCE(org_id::text, '00000000-0000-0000-0000-000000000000') = $2))`
		args = append(args, orgID)
	}
	err := s.db.QueryRow(query, args...).Scan(&w.ID, &name, &w.OrgID, &created)
	if err != nil {
		return nil, err
	}
	w.Name = name.String
	if created.Valid {
		w.CreatedAt = created.String
	}
	return &w, nil
}

func (s *Store) UpdateWorkspace(id string, w *Workspace, orgID string) (*Workspace, error) {
	err := s.db.QueryRow(`
		UPDATE workspaces SET name=$1, org_id=$2 WHERE id=$3
		RETURNING id, name, org_id, created_at`,
		w.Name, w.OrgID, id,
	).Scan(&w.ID, &w.Name, &w.OrgID, &w.CreatedAt)
	if err != nil {
		return nil, err
	}
	return w, nil
}

func (s *Store) DeleteWorkspace(id string, orgID string) error {
	_, err := s.db.Exec(`DELETE FROM workspaces WHERE id=$1 AND ((($2 = '' AND org_id IS NULL) OR ($2 = '' AND org_id = '00000000-0000-0000-0000-000000000000') OR COALESCE(org_id::text, '00000000-0000-0000-0000-000000000000') = $2))`, id, orgID)
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
		w.WorkspaceName = name.String
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

func jsonMarshal(v interface{}) ([]byte, error) {
	if v == nil {
		return []byte("[]"), nil
	}
	return json.Marshal(v)
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
	row := s.db.QueryRow(`SELECT id, username, password_hash, display_name, email, role, enabled, created_at, updated_at, COALESCE(org_id, '00000000-0000-0000-0000-000000000000') FROM users WHERE username = $1 AND enabled = true`, username)
	var u User
	var displayName, email sql.NullString
	err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &displayName, &email, &u.Role, &u.Enabled, &u.CreatedAt, &u.UpdatedAt, &u.OrgID)
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
	orgID := u.OrgID
	if orgID == "" {
		orgID = "00000000-0000-0000-0000-000000000000"
	}
	err := s.db.QueryRow(`INSERT INTO users (username, password_hash, display_name, email, role, org_id) VALUES ($1, $2, $3, $4, $5, $6) RETURNING id, created_at, updated_at`,
		u.Username, u.PasswordHash, u.DisplayName, u.Email, u.Role, orgID).Scan(&u.ID, &u.CreatedAt, &u.UpdatedAt)
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
	row := s.db.QueryRow(`SELECT id, username, password_hash, display_name, email, role, enabled, created_at, updated_at, last_login_at FROM users WHERE id = $1`, id)
	var u User
	if err := row.Scan(&u.ID, &u.Username, &u.PasswordHash, &u.DisplayName, &u.Email, &u.Role, &u.Enabled, &u.CreatedAt, &u.UpdatedAt, &u.LastLoginAt); err == sql.ErrNoRows {
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

func (s *Store) UpdateUserLastLogin(id string) error {
	_, err := s.db.Exec(`UPDATE users SET last_login_at=NOW() WHERE id=$1`, id)
	return err
}

// ── Organizations (SaaS multi-tenancy) ───────────────────────────────────

func (s *Store) CreateOrganization(org *Organization) (*Organization, error) {
	err := s.db.QueryRow(`
		INSERT INTO organizations (name, plan) VALUES ($1, $2)
		RETURNING id, created_at, updated_at
	`, org.Name, org.Plan).Scan(&org.ID, &org.CreatedAt, &org.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return org, nil
}

func (s *Store) GetOrganization(id string) (*Organization, error) {
	row := s.db.QueryRow(`SELECT id, name, plan, created_at, updated_at FROM organizations WHERE id=$1`, id)
	var org Organization
	err := row.Scan(&org.ID, &org.Name, &org.Plan, &org.CreatedAt, &org.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &org, nil
}

func (s *Store) GetOrganizationByName(name string) (*Organization, error) {
	row := s.db.QueryRow(`SELECT id, name, plan, created_at, updated_at FROM organizations WHERE name=$1`, name)
	var org Organization
	err := row.Scan(&org.ID, &org.Name, &org.Plan, &org.CreatedAt, &org.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &org, nil
}

func (s *Store) ListOrganizations() ([]Organization, error) {
	rows, err := s.db.Query(`SELECT id, name, plan, created_at, updated_at FROM organizations ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []Organization
	for rows.Next() {
		var o Organization
		if err := rows.Scan(&o.ID, &o.Name, &o.Plan, &o.CreatedAt, &o.UpdatedAt); err != nil {
			return nil, err
		}
		out = append(out, o)
	}
	return out, rows.Err()
}

// GetOrganizationByUserID returns the organization for a given user (via org_id column)
func (s *Store) GetOrganizationByUserID(userID string) (*Organization, error) {
	row := s.db.QueryRow(`
		SELECT o.id, o.name, o.plan, o.created_at, o.updated_at
		FROM organizations o
		JOIN users u ON u.org_id = o.id
		WHERE u.id = $1`, userID)
	var org Organization
	err := row.Scan(&org.ID, &org.Name, &org.Plan, &org.CreatedAt, &org.UpdatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &org, nil
}

// SetUserOrganization updates a user's org_id
func (s *Store) SetUserOrganization(userID, orgID string) error {
	_, err := s.db.Exec(`UPDATE users SET org_id=$1, updated_at=NOW() WHERE id=$2`, orgID, userID)
	return err
}

// ── OTP for email verification ───────────────────────────────────────────

func (s *Store) CreateOTP(email, code, purpose string, expiresInMinutes int) (*OTP, error) {
	// Delete any existing unverified OTP for this email+purpose
	s.db.Exec(`DELETE FROM otps WHERE email=$1 AND purpose=$2 AND verified=false`, email, purpose)

	var otp OTP
	err := s.db.QueryRow(`
		INSERT INTO otps (email, code, purpose, expires_at)
		VALUES ($1, $2, $3, NOW() + $4 * INTERVAL '1 minute')
		RETURNING id, email, code, purpose, expires_at, verified, created_at
	`, email, code, purpose, expiresInMinutes).Scan(
		&otp.ID, &otp.Email, &otp.Code, &otp.Purpose, &otp.ExpiresAt, &otp.Verified, &otp.CreatedAt)
	if err != nil {
		return nil, err
	}
	return &otp, nil
}

func (s *Store) GetOTP(email, code, purpose string) (*OTP, error) {
	row := s.db.QueryRow(`
		SELECT id, email, code, purpose, expires_at, verified, created_at
		FROM otps
		WHERE email=$1 AND purpose=$2 AND verified=false
		AND expires_at > NOW()
		ORDER BY created_at DESC
		LIMIT 1
	`, email, purpose)
	var otp OTP
	err := row.Scan(&otp.ID, &otp.Email, &otp.Code, &otp.Purpose, &otp.ExpiresAt, &otp.Verified, &otp.CreatedAt)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	// Verify code matches
	if otp.Code != code {
		return nil, nil
	}
	return &otp, nil
}

func (s *Store) MarkOTPVerified(id int64) error {
	_, err := s.db.Exec(`UPDATE otps SET verified=true WHERE id=$1`, id)
	return err
}

func (s *Store) DeleteOTP(id int64) error {
	_, err := s.db.Exec(`DELETE FROM otps WHERE id=$1`, id)
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

// AuditLogFilter holds filter criteria for ListAuditLogsFiltered
type AuditLogFilter struct {
	StartTime  *time.Time
	EndTime    *time.Time
	AuditType  string
	TargetType string
	Actor      string
	Limit      int
	Offset     int
}

func (s *Store) ListAuditLogsFiltered(f AuditLogFilter) ([]AuditLog, int, error) {
	where, args := []string{}, []interface{}{}
	argIdx := 1

	if f.StartTime != nil {
		where = append(where, fmt.Sprintf("created_at >= $%d", argIdx))
		args = append(args, *f.StartTime)
		argIdx++
	}
	if f.EndTime != nil {
		where = append(where, fmt.Sprintf("created_at <= $%d", argIdx))
		args = append(args, *f.EndTime)
		argIdx++
	}
	if f.AuditType != "" && f.AuditType != "all" {
		where = append(where, fmt.Sprintf("audit_type = $%d", argIdx))
		args = append(args, f.AuditType)
		argIdx++
	}
	if f.TargetType != "" && f.TargetType != "all" {
		where = append(where, fmt.Sprintf("target_type = $%d", argIdx))
		args = append(args, f.TargetType)
		argIdx++
	}
	if f.Actor != "" {
		where = append(where, fmt.Sprintf("(actor_username ILIKE $%d OR actor_user_id = $%d)", argIdx, argIdx))
		args = append(args, "%"+f.Actor+"%")
		argIdx++
	}

	whereClause := ""
	if len(where) > 0 {
		whereClause = "WHERE " + strings.Join(where, " AND ")
	}

	// Count total
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM audit_logs %s`, whereClause)
	var total int
	if err := s.db.QueryRow(countQuery, args...).Scan(&total); err != nil {
		return nil, 0, err
	}

	// Fetch rows
	query := fmt.Sprintf(`SELECT id, audit_type, target_type, target_id, actor_user_id, actor_username, description, created_at
		FROM audit_logs %s ORDER BY created_at DESC LIMIT $%d OFFSET $%d`,
		whereClause, argIdx, argIdx+1)
	args = append(args, f.Limit, f.Offset)

	rows, err := s.db.Query(query, args...)
	if err != nil {
		return nil, 0, err
	}
	defer rows.Close()
	var logs []AuditLog
	for rows.Next() {
		var l AuditLog
		var actorUID, actorUname sql.NullString
		if err := rows.Scan(&l.ID, &l.AuditType, &l.TargetType, &l.TargetID, &actorUID, &actorUname, &l.Description, &l.CreatedAt); err != nil {
			return nil, 0, err
		}
		if actorUID.Valid {
			l.ActorUserID = actorUID.String
		}
		if actorUname.Valid {
			l.ActorUsername = actorUname.String
		}
		logs = append(logs, l)
	}
	if logs == nil {
		logs = []AuditLog{}
	}
	return logs, total, nil
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
		`SELECT id, name, description, conditions, metric_type, service_name, threshold_value, operator,
		        duration_seconds, enabled, notification_channels, slack_webhook_url,
		        email_webhook_url, discord_webhook_url, alert_suppress_seconds,
		        last_triggered_at, last_triggered_value, created_at, updated_at
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
		var lastTriggeredAt sql.NullString
		var lastTriggeredValue sql.NullFloat64
		var conditionsJSON []byte
		if err := rows.Scan(&r.ID, &r.Name, &desc, &conditionsJSON, &r.MetricType, &svcName, &r.ThresholdValue, &r.Operator,
			&r.DurationSeconds, &r.Enabled, &notifCh, &slackURL, &emailURL, &discordURL,
			&r.AlertSuppressSeconds, &lastTriggeredAt, &lastTriggeredValue, &createdAt, &updatedAt); err != nil {
			return nil, err
		}
		if desc.Valid {
			r.Description = desc.String
		}
		if len(conditionsJSON) > 0 {
			json.Unmarshal(conditionsJSON, &r.Conditions)
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
		if lastTriggeredAt.Valid {
			r.LastTriggeredAt = &lastTriggeredAt.String
		}
		if lastTriggeredValue.Valid {
			r.LastTriggeredValue = &lastTriggeredValue.Float64
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
	conditionsJSON, _ := json.Marshal(r.Conditions)
	err := s.db.QueryRow(
		`INSERT INTO alert_rules (name, description, conditions, metric_type, service_name, threshold_value, operator,
		 duration_seconds, enabled, notification_channels, slack_webhook_url, email_webhook_url,
		 discord_webhook_url, alert_suppress_seconds) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14)
		 RETURNING id, created_at, updated_at`,
		r.Name, nullString(r.Description), conditionsJSON, r.MetricType, nullString(r.ServiceName), r.ThresholdValue,
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
	var conditionsJSON []byte
	err := s.db.QueryRow(
		`SELECT id, name, description, conditions, metric_type, service_name, threshold_value, operator,
		        duration_seconds, enabled, notification_channels, slack_webhook_url,
		        email_webhook_url, discord_webhook_url, alert_suppress_seconds, created_at, updated_at
		 FROM alert_rules WHERE id=$1`, id,
	).Scan(&r.ID, &r.Name, &desc, &conditionsJSON, &r.MetricType, &svcName, &r.ThresholdValue, &r.Operator,
		&r.DurationSeconds, &r.Enabled, &notifCh, &slackURL, &emailURL, &discordURL,
		&r.AlertSuppressSeconds, &createdAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	if desc.Valid {
		r.Description = desc.String
	}
	if len(conditionsJSON) > 0 {
		json.Unmarshal(conditionsJSON, &r.Conditions)
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
	conditionsJSON, _ := json.Marshal(r.Conditions)
	err := s.db.QueryRow(
		`UPDATE alert_rules SET name=$2, description=$3, conditions=$4, metric_type=$5, service_name=$6, threshold_value=$7,
		 operator=$8, duration_seconds=$9, enabled=$10, notification_channels=$11, slack_webhook_url=$12,
		 email_webhook_url=$13, discord_webhook_url=$14, alert_suppress_seconds=$15, updated_at=NOW()
		 WHERE id=$1 RETURNING updated_at`,
		id, r.Name, nullString(r.Description), conditionsJSON, r.MetricType, nullString(r.ServiceName), r.ThresholdValue,
		r.Operator, r.DurationSeconds, r.Enabled, nullString(r.NotificationChannels),
		nullString(r.SlackWebhookURL), nullString(r.EmailWebhookURL), nullString(r.DiscordWebhookURL),
		r.AlertSuppressSeconds,
	).Scan(&r.UpdatedAt)
	if err != nil {
		return nil, err
	}
	return r, nil
}

// UpdateAlertRuleTriggered records the last triggered timestamp and value for an alert rule.
// Called by the Alert Engine when a rule fires.
func (s *Store) UpdateAlertRuleTriggered(id int64, triggeredAt string, triggeredValue float64) error {
	_, err := s.db.Exec(
		`UPDATE alert_rules SET last_triggered_at=$2, last_triggered_value=$3, updated_at=NOW()
		 WHERE id=$1`,
		id, triggeredAt, triggeredValue)
	return err
}

func (s *Store) DeleteAlertRule(id string) error {
	_, err := s.db.Exec(`DELETE FROM alert_rules WHERE id=$1`, id)
	return err
}

// ── Alert History ──────────────────────────────────────────────────────────

func (s *Store) ListAlertHistory(limit, offset int) ([]AlertHistory, error) {
	rows, err := s.db.Query(
		`SELECT id, rule_id, rule_name, org_id, metric_type, operator, threshold, actual_value, triggered_at, COALESCE(message,'')
		 FROM alert_history ORDER BY triggered_at DESC LIMIT $1 OFFSET $2`, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	history := []AlertHistory{}
	for rows.Next() {
		var h AlertHistory
		var orgID sql.NullString
		if err := rows.Scan(&h.ID, &h.RuleID, &h.RuleName, &orgID, &h.MetricType, &h.Operator, &h.Threshold, &h.ActualValue, &h.TriggeredAt, &h.Message); err != nil {
			return nil, err
		}
		if orgID.Valid {
			h.OrgID = &orgID.String
		}
		history = append(history, h)
	}
	return history, rows.Err()
}

func (s *Store) CreateAlertHistory(h *AlertHistory) error {
	row := s.db.QueryRow(
		`INSERT INTO alert_history(rule_id, rule_name, org_id, metric_type, operator, threshold, actual_value, message)
		 VALUES($1,$2,$3,$4,$5,$6,$7,$8) RETURNING id, triggered_at`,
		h.RuleID, h.RuleName, h.OrgID, h.MetricType, h.Operator, h.Threshold, h.ActualValue, h.Message)
	return row.Scan(&h.ID, &h.TriggeredAt)
}

// ── API Key Requests ───────────────────────────────────────────────────────

func (s *Store) ListAPIKeyRequests() ([]APIKeyRequest, error) {
	rows, err := s.db.Query(
		`SELECT id, key_name, consumer_name, description, status, applicant_user_id, applicant_username,
		        reviewed_by, reviewed_at, generated_key, created_at, updated_at
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
		var generatedKey sql.NullString
		if err := rows.Scan(&r.ID, &r.KeyName, &consumerName, &desc, &r.Status,
			&applicantUserID, &applicantUsername, &reviewedBy, &reviewedAt, &generatedKey, &createdAt, &updatedAt); err != nil {
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
		if generatedKey.Valid {
			r.GeneratedKey = generatedKey.String
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
		`INSERT INTO api_key_requests (key_name, consumer_name, description, reason, scopes, expires_at, status, applicant_user_id, applicant_username)
		 VALUES ($1,$2,$3,$4,$5,$6,'pending',$7,$8) RETURNING id, created_at, updated_at`,
		r.KeyName, nullString(r.ConsumerName), nullString(r.Description),
		nullString(r.Reason), nullString(r.Scopes), nullString(r.ExpiresAt),
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
	var generatedKey, reason, scopes, expiresAt, keyValue sql.NullString
	err = s.db.QueryRow(
		`SELECT id, key_name, consumer_name, description, reason, scopes, expires_at, status,
		        applicant_user_id, applicant_username, reviewed_by, reviewed_at,
		        generated_key, key_value, created_at, updated_at
		 FROM api_key_requests WHERE id=$1`, intID,
	).Scan(&r.ID, &r.KeyName, &consumerName, &desc, &reason, &scopes, &expiresAt, &r.Status,
		&applicantUserID, &applicantUsername, &reviewedBy, &reviewedAt, &generatedKey, &keyValue, &createdAt, &updatedAt)
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
	if generatedKey.Valid {
		r.GeneratedKey = generatedKey.String
	}
	if keyValue.Valid {
		r.KeyValue = keyValue.String
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
		 reviewed_by=$6, reviewed_at=NOW(), generated_key=$7, updated_at=NOW(),
		 reason=$8, scopes=$9, expires_at=$10, key_value=$11 WHERE id=$1`,
		intID, r.KeyName, nullString(r.ConsumerName), nullString(r.Description), r.Status,
		nullString(r.ReviewedBy), nullString(r.GeneratedKey),
		nullString(r.Reason), nullString(r.Scopes), nullString(r.ExpiresAt), nullString(r.KeyValue),
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
		`SELECT id, version_label, config_data, diff_from_prev, actor_user_id, actor_username, created_at
		 FROM config_snapshots ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var snaps []ConfigSnapshot
	for rows.Next() {
		var sn ConfigSnapshot
		var configData, diffFromPrev sql.NullString
		var actorUID, actorUname sql.NullString
		var createdAt sql.NullString
		if err := rows.Scan(&sn.ID, &sn.VersionLabel, &configData, &diffFromPrev, &actorUID, &actorUname, &createdAt); err != nil {
			return nil, err
		}
		if configData.Valid {
			sn.ConfigData = &configData.String
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
	// Capture current Kong config (services, routes, plugins, consumers)
	// as JSON config_data for rollback
	configData, err := s.captureCurrentConfig()
	if err != nil {
		configData = "{}" // non-fatal, continue without
	}

	// Compute diff_from_prev by comparing with most recent snapshot
	var diffFromPrev *string
	prevSnaps, err := s.ListConfigSnapshots()
	if err == nil && len(prevSnaps) > 0 && prevSnaps[0].ConfigData != nil {
		diff := ComputeConfigDiff(*prevSnaps[0].ConfigData, configData)
		if diff != "{}" {
			diffFromPrev = &diff
		}
	}

	var outID int64
	err = s.db.QueryRow(
		`INSERT INTO config_snapshots (version_label, config_data, diff_from_prev, actor_user_id, actor_username)
		 VALUES ($1,$2,$3,$4,$5) RETURNING id, created_at`,
		sn.VersionLabel, configData, nullString(stringVal(diffFromPrev)), nullString(sn.ActorUserID), nullString(sn.ActorUsername),
	).Scan(&outID, &sn.CreatedAt)
	if err != nil {
		return nil, err
	}
	sn.ID = outID
	sn.ConfigData = &configData
	sn.DiffFromPrev = diffFromPrev
	return sn, nil
}

// captureCurrentConfig grabs all current services/routes/plugins/consumers from DB
func (s *Store) captureCurrentConfig() (string, error) {
	type cfg struct {
		Services  []map[string]interface{} `json:"services"`
		Routes    []map[string]interface{} `json:"routes"`
		Plugins   []map[string]interface{} `json:"plugins"`
		Consumers []map[string]interface{} `json:"consumers"`
	}

	// Services
	svcs, err := s.ListServices("00000000-0000-0000-0000-000000000000", 0, 0)
	if err != nil {
		return "{}", err
	}
	svcMaps := make([]map[string]interface{}, 0, len(svcs))
	for _, svc := range svcs {
		svcMaps = append(svcMaps, map[string]interface{}{
			"id":   svc.ID,
			"name": svc.Name,
			"host": svc.Host,
			"port": svc.Port,
			"path": svc.Path,
			"protocol": svc.Protocol,
		})
	}

	// Routes (admin only, no org filter)
	routes, err := s.ListRoutes("00000000-0000-0000-0000-000000000000", 0, 0)
	if err != nil {
		return "{}", err
	}
	routeMaps := make([]map[string]interface{}, 0, len(routes))
	for _, r := range routes {
		routeMaps = append(routeMaps, map[string]interface{}{
			"id":   r.ID,
			"name": r.Name,
			"service_id": r.GetServiceID(),
			"hosts": r.Hosts,
			"paths": r.Paths,
		})
	}

	// Plugins (admin only)
	plugins, err := s.ListPlugins("00000000-0000-0000-0000-000000000000", 0, 0)
	if err != nil {
		return "{}", err
	}
	pluginMaps := make([]map[string]interface{}, 0, len(plugins))
	for _, p := range plugins {
		pluginMaps = append(pluginMaps, map[string]interface{}{
			"id":   p.ID,
			"name": p.Name,
			"scope": p.Scope,
			"enabled": p.Enabled,
		})
	}

	// Consumers (admin only)
	consumers, err := s.ListConsumers("00000000-0000-0000-0000-000000000000", 0, 0)
	if err != nil {
		return "{}", err
	}
	consMaps := make([]map[string]interface{}, 0, len(consumers))
	for _, c := range consumers {
		consMaps = append(consMaps, map[string]interface{}{
			"id":   c.ID,
			"username": c.Username,
		})
	}

	c := cfg{
		Services:  svcMaps,
		Routes:   routeMaps,
		Plugins:  pluginMaps,
		Consumers: consMaps,
	}
	out, err := json.Marshal(c)
	if err != nil {
		return "{}", err
	}
	return string(out), nil
}

// ComputeConfigDiff returns a JSON string describing changes between two config JSON strings
func ComputeConfigDiff(prev, current string) string {
	type item struct {
		ID   string `json:"id,omitempty"`
		Name string `json:"name,omitempty"`
	}
	type change struct {
		Op    string                 `json:"op"`
		Name  string                 `json:"name"`
		Item  map[string]interface{}  `json:"item,omitempty"`
		Changes map[string]struct { From, To interface{} } `json:"changes,omitempty"`
	}

	var p, cur struct {
		Services  []map[string]interface{} `json:"services"`
		Routes    []map[string]interface{} `json:"routes"`
		Plugins   []map[string]interface{} `json:"plugins"`
		Consumers []map[string]interface{} `json:"consumers"`
	}
	if err := json.Unmarshal([]byte(prev), &p); err != nil {
		return "{}"
	}
	if err := json.Unmarshal([]byte(current), &cur); err != nil {
		return "{}"
	}

	result := struct {
		Services  []change `json:"services"`
		Routes    []change `json:"routes"`
		Plugins   []change `json:"plugins"`
		Consumers []change `json:"consumers"`
	}{}

	// Services diff
	prevSvc := map[string]map[string]interface{}{}
	for _, s := range p.Services {
		if id, ok := s["id"].(string); ok {
			prevSvc[id] = s
		}
	}
	for _, s := range cur.Services {
		id, _ := s["id"].(string)
		name, _ := s["name"].(string)
		if name == "" {
			name = id
		}
		if old, exists := prevSvc[id]; !exists {
			result.Services = append(result.Services, change{Op: "add", Name: name, Item: s})
		} else {
			changes := map[string]struct{ From, To interface{} }{}
			for k, v := range s {
				if k == "id" {
					continue
				}
				if ov, ok := old[k]; ok && !reflect.DeepEqual(ov, v) {
					changes[k] = struct{ From, To interface{} }{From: ov, To: v}
				}
			}
			if len(changes) > 0 {
				result.Services = append(result.Services, change{Op: "update", Name: name, Changes: changes})
			}
			delete(prevSvc, id)
		}
	}
	for _, s := range prevSvc {
		name, _ := s["name"].(string)
		id, _ := s["id"].(string)
		if name == "" {
			name = id
		}
		result.Services = append(result.Services, change{Op: "delete", Name: name})
	}

	// Routes diff (similar pattern)
	prevR := map[string]map[string]interface{}{}
	for _, r := range p.Routes {
		if id, ok := r["id"].(string); ok {
			prevR[id] = r
		}
	}
	for _, r := range cur.Routes {
		id, _ := r["id"].(string)
		name, _ := r["name"].(string)
		if name == "" {
			name = id
		}
		if _, exists := prevR[id]; !exists {
			result.Routes = append(result.Routes, change{Op: "add", Name: name, Item: r})
		} else {
			delete(prevR, id)
		}
	}
	for _, r := range prevR {
		name, _ := r["name"].(string)
		id, _ := r["id"].(string)
		if name == "" {
			name = id
		}
		result.Routes = append(result.Routes, change{Op: "delete", Name: name})
	}

	// Plugins diff
	prevPl := map[string]map[string]interface{}{}
	for _, pl := range p.Plugins {
		if id, ok := pl["id"].(string); ok {
			prevPl[id] = pl
		}
	}
	for _, pl := range cur.Plugins {
		id, _ := pl["id"].(string)
		name, _ := pl["name"].(string)
		if _, exists := prevPl[id]; !exists {
			result.Plugins = append(result.Plugins, change{Op: "add", Name: name, Item: pl})
		} else {
			delete(prevPl, id)
		}
	}
	for _, pl := range prevPl {
		name, _ := pl["name"].(string)
		result.Plugins = append(result.Plugins, change{Op: "delete", Name: name})
	}

	// Consumers diff
	prevC := map[string]map[string]interface{}{}
	for _, c := range p.Consumers {
		if id, ok := c["id"].(string); ok {
			prevC[id] = c
		}
	}
	for _, c := range cur.Consumers {
		id, _ := c["id"].(string)
		name, _ := c["username"].(string)
		if _, exists := prevC[id]; !exists {
			result.Consumers = append(result.Consumers, change{Op: "add", Name: name, Item: c})
		} else {
			delete(prevC, id)
		}
	}
	for _, c := range prevC {
		name, _ := c["username"].(string)
		result.Consumers = append(result.Consumers, change{Op: "delete", Name: name})
	}

	out, _ := json.Marshal(result)
	return string(out)
}

func (s *Store) GetConfigSnapshot(id string) (*ConfigSnapshot, error) {
	var sn ConfigSnapshot
	var configData, diffFromPrev sql.NullString
	var actorUID, actorUname sql.NullString
	var createdAt sql.NullString
	err := s.db.QueryRow(
		`SELECT id, version_label, config_data, diff_from_prev, actor_user_id, actor_username, created_at
		 FROM config_snapshots WHERE id=$1`, id,
	).Scan(&sn.ID, &sn.VersionLabel, &configData, &diffFromPrev, &actorUID, &actorUname, &createdAt)
	if err != nil {
		return nil, err
	}
	if configData.Valid {
		sn.ConfigData = &configData.String
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

// DiffConfigSnapshots returns a diff between two snapshots by ID
func (s *Store) DiffConfigSnapshots(id1, id2 string) (string, error) {
	s1, err := s.GetConfigSnapshot(id1)
	if err != nil {
		return "{}", fmt.Errorf("snapshot %s not found: %w", id1, err)
	}
	s2, err := s.GetConfigSnapshot(id2)
	if err != nil {
		return "{}", fmt.Errorf("snapshot %s not found: %w", id2, err)
	}
	if s1.ConfigData == nil || s2.ConfigData == nil {
		return "{}", fmt.Errorf("snapshot config data not available")
	}
	return ComputeConfigDiff(*s1.ConfigData, *s2.ConfigData), nil
}

// RollbackConfigSnapshot restores Kong config from a snapshot
// Returns a list of errors encountered during restore
func (s *Store) RollbackConfigSnapshot(id string) (map[string][]string, error) {
	snap, err := s.GetConfigSnapshot(id)
	if err != nil {
		return nil, fmt.Errorf("snapshot not found: %w", err)
	}
	if snap.ConfigData == nil {
		return nil, fmt.Errorf("snapshot has no config data to restore")
	}

	var cfg struct {
		Services  []map[string]interface{} `json:"services"`
		Routes    []map[string]interface{} `json:"routes"`
		Plugins   []map[string]interface{} `json:"plugins"`
		Consumers []map[string]interface{} `json:"consumers"`
	}
	if err := json.Unmarshal([]byte(*snap.ConfigData), &cfg); err != nil {
		return nil, fmt.Errorf("invalid config data: %w", err)
	}

	errors := map[string][]string{}

	// Restore services: delete current, re-create from snapshot
	currentSvcs, _ := s.ListServices("00000000-0000-0000-0000-000000000000", 0, 0)
	for _, svc := range currentSvcs {
		if err := s.DeleteService(svc.ID, ""); err != nil {
			errors["services"] = append(errors["services"], fmt.Sprintf("delete %s: %v", svc.ID, err))
		}
	}
	for _, svc := range cfg.Services {
		svcRec := Service{
			Name:     getStr(svc, "name"),
			Host:     getStr(svc, "host"),
			Port:     getInt(svc, "port"),
			Path:     getStr(svc, "path"),
			Protocol: getStr(svc, "protocol"),
		}
		if _, err := s.CreateService(&svcRec); err != nil {
			errors["services"] = append(errors["services"], fmt.Sprintf("create %s: %v", svcRec.Name, err))
		}
	}

	// Restore consumers
	currentCons, _ := s.ListConsumers("00000000-0000-0000-0000-000000000000", 0, 0)
	for _, c := range currentCons {
		if err := s.DeleteConsumer(c.ID, ""); err != nil {
			errors["consumers"] = append(errors["consumers"], fmt.Sprintf("delete %s: %v", c.ID, err))
		}
	}
	for _, c := range cfg.Consumers {
		cu := Consumer{Username: getStr(c, "username")}
		if _, err := s.CreateConsumer(&cu); err != nil {
			errors["consumers"] = append(errors["consumers"], fmt.Sprintf("create %s: %v", cu.Username, err))
		}
	}

	// Restore routes
	currentRoutes, _ := s.ListRoutes("00000000-0000-0000-0000-000000000000", 0, 0)
	for _, r := range currentRoutes {
		if err := s.DeleteRoute(r.ID, ""); err != nil {
			errors["routes"] = append(errors["routes"], fmt.Sprintf("delete %s: %v", r.ID, err))
		}
	}
	for _, r := range cfg.Routes {
		route := Route{
			Name: getStr(r, "name"),
			Service: &ServiceRef{ID: getStr(r, "service_id")},
			Hosts:    getStrArr(r, "hosts"),
			Paths:    getStrArr(r, "paths"),
		}
		if _, err := s.CreateRoute(&route); err != nil {
			errors["routes"] = append(errors["routes"], fmt.Sprintf("create %s: %v", route.Name, err))
		}
	}

	// Restore plugins
	currentPlugins, _ := s.ListPlugins("00000000-0000-0000-0000-000000000000", 0, 0)
	for _, p := range currentPlugins {
		if err := s.DeletePlugin(p.ID, ""); err != nil {
			errors["plugins"] = append(errors["plugins"], fmt.Sprintf("delete %s: %v", p.ID, err))
		}
	}
	for _, p := range cfg.Plugins {
		pl := Plugin{
			Name:    getStr(p, "name"),
			Enabled: getBool(p, "enabled"),
			Scope:   getStr(p, "scope"),
		}
		if _, err := s.CreatePlugin(&pl); err != nil {
			errors["plugins"] = append(errors["plugins"], fmt.Sprintf("create %s: %v", pl.Name, err))
		}
	}

	return errors, nil
}

// Helpers
func getStr(m map[string]interface{}, k string) string {
	if v, ok := m[k].(string); ok {
		return v
	}
	return ""
}
func getInt(m map[string]interface{}, k string) int {
	if v, ok := m[k].(float64); ok {
		return int(v)
	}
	return 0
}
func getBool(m map[string]interface{}, k string) bool {
	if v, ok := m[k].(bool); ok {
		return v
	}
	return false
}
func getStrArr(m map[string]interface{}, k string) []string {
	if v, ok := m[k].([]interface{}); ok {
		r := make([]string, 0, len(v))
		for _, i := range v {
			if s, ok := i.(string); ok {
				r = append(r, s)
			}
		}
		return r
	}
	return nil
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

// ── Resource Permissions ────────────────────────────────────────────────────

// ListUserResourcePermissions returns all resource permissions for a user (with resource names)
func (s *Store) ListUserResourcePermissions(userID string) ([]ResourcePermission, error) {
	rows, err := s.db.Query(`
		SELECT rp.subject_type, rp.subject_id, rp.resource_id, rp.permission, rp.created_at, r.name
		FROM resource_permissions rp
		JOIN resources r ON r.id = rp.resource_id
		WHERE rp.subject_type = 'user' AND rp.subject_id = $1
		ORDER BY r.name`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var perms []ResourcePermission
	for rows.Next() {
		var p ResourcePermission
		var createdAt sql.NullString
		var resourceName sql.NullString
		if err := rows.Scan(&p.SubjectType, &p.SubjectID, &p.ResourceID, &p.Permission, &createdAt, &resourceName); err != nil {
			return nil, err
		}
		if createdAt.Valid {
			p.CreatedAt = createdAt.String
		}
		if resourceName.Valid {
			p.ResourceName = resourceName.String
		}
		perms = append(perms, p)
	}
	return perms, rows.Err()
}

// SetUserResourcePermission upserts a resource permission for a user (empty permission = delete)
func (s *Store) SetUserResourcePermission(userID, resourceID, permission string) error {
	if permission == "" {
		_, err := s.db.Exec(`DELETE FROM resource_permissions WHERE subject_type='user' AND subject_id=$1 AND resource_id=$2`, userID, resourceID)
		return err
	}
	_, err := s.db.Exec(`
		INSERT INTO resource_permissions (subject_type, subject_id, resource_id, permission)
		VALUES ('user', $1, $2, $3)
		ON CONFLICT (subject_type, subject_id, resource_id) DO UPDATE SET permission = $3`,
		userID, resourceID, permission)
	return err
}

// ListGroupResourcePermissions returns all resource permissions for an auth group (with resource names)
func (s *Store) ListGroupResourcePermissions(authGroupID string) ([]ResourcePermission, error) {
	rows, err := s.db.Query(`
		SELECT rp.subject_type, rp.subject_id, rp.resource_id, rp.permission, rp.created_at, r.name
		FROM resource_permissions rp
		JOIN resources r ON r.id = rp.resource_id
		WHERE rp.subject_type = 'group' AND rp.subject_id = $1
		ORDER BY r.name`, authGroupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var perms []ResourcePermission
	for rows.Next() {
		var p ResourcePermission
		var createdAt sql.NullString
		var resourceName sql.NullString
		if err := rows.Scan(&p.SubjectType, &p.SubjectID, &p.ResourceID, &p.Permission, &createdAt, &resourceName); err != nil {
			return nil, err
		}
		if createdAt.Valid {
			p.CreatedAt = createdAt.String
		}
		if resourceName.Valid {
			p.ResourceName = resourceName.String
		}
		perms = append(perms, p)
	}
	return perms, rows.Err()
}

// SetGroupResourcePermission upserts a resource permission for an auth group (empty permission = delete)
func (s *Store) SetGroupResourcePermission(authGroupID, resourceID, permission string) error {
	if permission == "" {
		_, err := s.db.Exec(`DELETE FROM resource_permissions WHERE subject_type='group' AND subject_id=$1 AND resource_id=$2`, authGroupID, resourceID)
		return err
	}
	_, err := s.db.Exec(`
		INSERT INTO resource_permissions (subject_type, subject_id, resource_id, permission)
		VALUES ('group', $1, $2, $3)
		ON CONFLICT (subject_type, subject_id, resource_id) DO UPDATE SET permission = $3`,
		authGroupID, resourceID, permission)
	return err
}

// ── Resources (full CRUD for resource-level RBAC) ────────────────────────

func (s *Store) GetResource(id string) (*Resource, error) {
	var r Resource
	var typ sql.NullString
	err := s.db.QueryRow(`SELECT id, name, path, type FROM resources WHERE id = $1`, id).
		Scan(&r.ID, &r.Name, &r.Path, &typ)
	if err != nil {
		return nil, err
	}
	if typ.Valid {
		r.Type = typ.String
	}
	return &r, nil
}

func (s *Store) CreateResource(r *Resource) (*Resource, error) {
	var typ sql.NullString
	err := s.db.QueryRow(`
		INSERT INTO resources (id, name, path, type)
		VALUES ($1, $2, $3, $4)
		RETURNING id, name, path, type`,
		r.ID, r.Name, r.Path, sql.NullString{String: r.Type, Valid: r.Type != ""},
	).Scan(&r.ID, &r.Name, &r.Path, &typ)
	if err != nil {
		return nil, err
	}
	if typ.Valid {
		r.Type = typ.String
	}
	return r, nil
}

func (s *Store) DeleteResource(id string) error {
	_, err := s.db.Exec(`DELETE FROM resources WHERE id = $1`, id)
	return err
}

// ListAllTargets returns all targets (across all upstreams) for resource enumeration
func (s *Store) ListAllTargets() ([]Target, error) {
	rows, err := s.db.Query(`SELECT id, upstream_id, target, weight, enabled FROM targets ORDER BY upstream_id, target`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var targets []Target
	for rows.Next() {
		var t Target
		var enabled sql.NullBool
		if err := rows.Scan(&t.ID, &t.UpstreamID, &t.Target, &t.Weight, &enabled); err != nil {
			return nil, err
		}
		if enabled.Valid {
			t.Enabled = enabled.Bool
		}
		targets = append(targets, t)
	}
	return targets, nil
}

// GetResourcePermissionsForUser returns effective permissions for a user on a specific resource
// It checks both user-level and group-level resource permissions
func (s *Store) GetResourcePermissionsForUser(userID, resourceID string) (string, error) {
	// First check direct user permissions
	row := s.db.QueryRow(`SELECT permission FROM resource_permissions
		WHERE subject_type='user' AND subject_id=$1 AND resource_id=$2`, userID, resourceID)
	var perm string
	if err := row.Scan(&perm); err == nil {
		return perm, nil
	}
	// Then check group memberships
	rows, err := s.db.Query(`SELECT rp.permission FROM resource_permissions rp
		JOIN user_auth_groups uag ON rp.subject_id = uag.auth_group_id
		WHERE rp.subject_type='group' AND uag.user_id=$1 AND rp.resource_id=$2`, userID, resourceID)
	if err != nil {
		return "", err
	}
	defer rows.Close()
	for rows.Next() {
		var perm string
		if err := rows.Scan(&perm); err == nil && perm != "" {
			return perm, nil
		}
	}
	return "", nil
}

// GetResourcePermissionsForGroup returns effective permissions for an auth group on a specific resource
func (s *Store) GetResourcePermissionsForGroup(authGroupID, resourceID string) (string, error) {
	row := s.db.QueryRow(`SELECT permission FROM resource_permissions
		WHERE subject_type='group' AND subject_id=$1 AND resource_id=$2`, authGroupID, resourceID)
	var perm string
	err := row.Scan(&perm)
	return perm, err
}

// ── OAuth Provider Management ───────────────────────────────────────────────

// ListOAuthProviders returns all configured OAuth providers
func (s *Store) ListOAuthProviders() ([]OAuthProviderModel, error) {
	rows, err := s.db.Query(`
		SELECT id, provider, client_id, client_secret, issuer_url,
		       authorization_url, token_url, userinfo_url, jwks_url,
		       scopes, enabled, created_at, updated_at
		FROM oauth_providers ORDER BY provider`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var providers []OAuthProviderModel
	for rows.Next() {
		var p OAuthProviderModel
		var issURL, authURL, userInfoURL, jwksURL, scopes, updatedAt sql.NullString
		err := rows.Scan(&p.ID, &p.Provider, &p.ClientID, &p.ClientSecret,
			&issURL, &authURL, &p.TokenURL, &userInfoURL, &jwksURL,
			&scopes, &p.Enabled, &p.CreatedAt, &updatedAt)
		if err != nil {
			return nil, err
		}
		p.IssuerURL = issURL.String
		p.AuthorizationURL = authURL.String
		p.UserInfoURL = userInfoURL.String
		p.JWKSURL = jwksURL.String
		p.Scopes = scopes.String
		p.UpdatedAt = updatedAt.String
		providers = append(providers, p)
	}
	return providers, nil
}

// GetOAuthProvider returns a single OAuth provider by name
func (s *Store) GetOAuthProvider(provider string) (*OAuthProviderModel, error) {
	var p OAuthProviderModel
	var issURL, authURL, userInfoURL, jwksURL, scopes, updatedAt sql.NullString
	err := s.db.QueryRow(`
		SELECT id, provider, client_id, client_secret, issuer_url,
		       authorization_url, token_url, userinfo_url, jwks_url,
		       scopes, enabled, created_at, updated_at
		FROM oauth_providers WHERE provider = $1`, provider,
	).Scan(&p.ID, &p.Provider, &p.ClientID, &p.ClientSecret,
		&issURL, &authURL, &p.TokenURL, &userInfoURL, &jwksURL,
		&scopes, &p.Enabled, &p.CreatedAt, &updatedAt)
	if err != nil {
		return nil, err
	}
	p.IssuerURL = issURL.String
	p.AuthorizationURL = authURL.String
	p.UserInfoURL = userInfoURL.String
	p.JWKSURL = jwksURL.String
	p.Scopes = scopes.String
	p.UpdatedAt = updatedAt.String
	return &p, nil
}

// CreateOAuthProvider creates a new OAuth provider
func (s *Store) CreateOAuthProvider(p *OAuthProviderModel) (*OAuthProviderModel, error) {
	if p.Scopes == "" {
		p.Scopes = "openid email profile"
	}
	var id string
	err := s.db.QueryRow(`
		INSERT INTO oauth_providers (provider, client_id, client_secret, issuer_url,
			authorization_url, token_url, userinfo_url, jwks_url, scopes, enabled)
		VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
		RETURNING id, created_at`,
		p.Provider, p.ClientID, p.ClientSecret, p.IssuerURL,
		p.AuthorizationURL, p.TokenURL, p.UserInfoURL, p.JWKSURL,
		p.Scopes, p.Enabled,
	).Scan(&id, &p.CreatedAt)
	if err != nil {
		return nil, err
	}
	p.ID = id
	return p, nil
}

// UpdateOAuthProvider updates an existing OAuth provider
func (s *Store) UpdateOAuthProvider(provider string, p *OAuthProviderModel) (*OAuthProviderModel, error) {
	result, err := s.db.Exec(`
		UPDATE oauth_providers SET
			client_id = $1,
			client_secret = CASE WHEN $2 != '' THEN $2 ELSE client_secret END,
			issuer_url = $3,
			authorization_url = $4,
			token_url = $5,
			userinfo_url = $6,
			jwks_url = $7,
			scopes = $8,
			enabled = $9,
			updated_at = NOW()
		WHERE provider = $10`,
		p.ClientID, p.ClientSecret, p.IssuerURL,
		p.AuthorizationURL, p.TokenURL, p.UserInfoURL, p.JWKSURL,
		p.Scopes, p.Enabled, provider,
	)
	if err != nil {
		return nil, err
	}
	rowsAff, _ := result.RowsAffected()
	if rowsAff == 0 {
		return nil, fmt.Errorf("provider not found")
	}
	return s.GetOAuthProvider(provider)
}

// DeleteOAuthProvider deletes an OAuth provider
func (s *Store) DeleteOAuthProvider(provider string) error {
	result, err := s.db.Exec(`DELETE FROM oauth_providers WHERE provider = $1`, provider)
	if err != nil {
		return err
	}
	rowsAff, _ := result.RowsAffected()
	if rowsAff == 0 {
		return fmt.Errorf("provider not found")
	}
	return nil
}

// SeedGoogleOAuthProvider seeds a Google OAuth provider if none exists
func (s *Store) SeedGoogleOAuthProvider() error {
	existing, _ := s.GetOAuthProvider("google")
	if existing != nil {
		return nil
	}
	_, err := s.CreateOAuthProvider(&OAuthProviderModel{
		Provider:         "google",
		ClientID:         "",
		ClientSecret:     "",
		IssuerURL:        "https://accounts.google.com",
		AuthorizationURL: "https://accounts.google.com/o/oauth2/v2/auth",
		TokenURL:         "https://oauth2.googleapis.com/token",
		UserInfoURL:      "https://openidconnect.googleapis.com/v1/userinfo",
		Scopes:           "openid email profile",
		Enabled:          false,
	})
	return err
}

// CreateNotification stores a notification record and broadcasts via SSE
func (s *Store) CreateNotification(n *Notification) (*Notification, error) {
	var id string
	err := s.db.QueryRow(`INSERT INTO notifications(user_id, type, payload, read, created_at) VALUES($1, $2, $3, $4, NOW()) RETURNING id`,
		n.UserID, n.Type, n.Payload, false).Scan(&id)
	if err != nil {
		return nil, err
	}
	n.ID = id
	n.Read = false
	payloadJSON, _ := json.Marshal(map[string]interface{}{
		"id":         n.ID,
		"type":       n.Type,
		"user_id":    n.UserID,
		"payload":    n.Payload,
		"created_at": n.CreatedAt,
	})
	Hub.BroadcastToUser(n.UserID, n.Type, json.RawMessage(payloadJSON))
	return n, nil
}

// ListNotifications returns notifications for a user (unread first)
func (s *Store) ListNotifications(userID string, limit, offset int) ([]Notification, error) {
	rows, err := s.db.Query(`SELECT id, user_id, type, payload, read, created_at FROM notifications WHERE user_id = $1 ORDER BY read ASC, created_at DESC LIMIT $2 OFFSET $3`, userID, limit, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var notifs []Notification
	for rows.Next() {
		var n Notification
		if err := rows.Scan(&n.ID, &n.UserID, &n.Type, &n.Payload, &n.Read, &n.CreatedAt); err != nil {
			return nil, err
		}
		notifs = append(notifs, n)
	}
	return notifs, nil
}

// MarkNotificationRead marks a notification as read
func (s *Store) MarkNotificationRead(id, userID string) error {
	_, err := s.db.Exec(`UPDATE notifications SET read = true WHERE id = $1 AND user_id = $2`, id, userID)
	return err
}

// MarkAllNotificationsRead marks all notifications as read for a user
func (s *Store) MarkAllNotificationsRead(userID string) error {
	_, err := s.db.Exec(`UPDATE notifications SET read = true WHERE user_id = $1`, userID)
	return err
}

// CountUnreadNotifications returns count of unread notifications
func (s *Store) CountUnreadNotifications(userID string) (int, error) {
	var count int
	err := s.db.QueryRow(`SELECT COUNT(*) FROM notifications WHERE user_id = $1 AND read = false`, userID).Scan(&count)
	return count, err
}
