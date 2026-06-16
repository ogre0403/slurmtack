package slurm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/slurmtack/slurmtack/internal/trace"
)

type Option func(*RestClient)

func WithHTTPClient(c *http.Client) Option {
	return func(rc *RestClient) { rc.httpClient = c }
}

func WithTimeout(d time.Duration) Option {
	return func(rc *RestClient) { rc.httpClient.Timeout = d }
}

func WithAMQPURL(url string) Option {
	return func(rc *RestClient) { rc.amqpURL = url }
}

func WithPlaceholderSIFPath(path string) Option {
	return func(rc *RestClient) { rc.placeholderSIFPath = path }
}

func WithSlurmUser(user string) Option {
	return func(rc *RestClient) { rc.slurmUser = user }
}

func WithAdminCredentials(user, token string) Option {
	return func(rc *RestClient) {
		rc.adminUser = user
		rc.adminToken = token
	}
}

func WithLogger(logger *slog.Logger) Option {
	return func(rc *RestClient) {
		rc.logger = trace.OrDefault(logger).With("component", "slurmrestd_client")
	}
}

type RestClient struct {
	baseURL            string
	jwtToken           string
	slurmUser          string
	adminUser          string
	adminToken         string
	logger             *slog.Logger
	httpClient         *http.Client
	amqpURL            string
	placeholderSIFPath string
}

func NewRestClient(baseURL, jwtToken string, opts ...Option) *RestClient {
	rc := &RestClient{
		baseURL:   strings.TrimRight(baseURL, "/"),
		jwtToken:  jwtToken,
		slurmUser: "cloud-user",
		logger:    trace.OrDefault(nil).With("component", "slurmrestd_client"),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
	for _, o := range opts {
		o(rc)
	}
	// Admin credentials default to workload credentials when not set.
	if rc.adminUser == "" {
		rc.adminUser = rc.slurmUser
	}
	if rc.adminToken == "" {
		rc.adminToken = rc.jwtToken
	}
	return rc
}

func (c *RestClient) SubmitPlaceholderJob(ctx context.Context, req PlaceholderJobRequest) (*PlaceholderJobResult, error) {
	effectiveUser := c.slurmUser
	effectiveToken := c.jwtToken
	if req.WorkloadUser != "" && req.WorkloadToken != "" {
		effectiveUser = req.WorkloadUser
		effectiveToken = req.WorkloadToken
	}

	homeDir := fmt.Sprintf("/home/%s", effectiveUser)
	resolvedSIFPath := filepath.Join(homeDir, c.placeholderSIFPath, req.PlaceholderSIFFile)
	resolvedSIFPath = filepath.Clean(resolvedSIFPath)

	script := fmt.Sprintf("#!/bin/bash\n#SBATCH --job-name=gpu-switch-%s\n#SBATCH --nodes=1\n#SBATCH --ntasks=1\n#SBATCH --exclusive=user\n", req.ExecutionID)
	if req.Constraint != "" {
		script += fmt.Sprintf("#SBATCH --constraint=%s\n", req.Constraint)
	}
	if req.Partition != "" {
		script += fmt.Sprintf("#SBATCH --partition=%s\n", req.Partition)
	}

	script += fmt.Sprintf("export EXECUTION_ID=%s\n", req.ExecutionID)
	script += fmt.Sprintf("export AMQP_URL=%s\n", c.amqpURL)
	script += fmt.Sprintf("export SLURM_API_URL=%s\n", c.baseURL)
	script += fmt.Sprintf("export SLURM_JWT_TOKEN=%s\n", effectiveToken)
	script += fmt.Sprintf("export SLURM_API_USER=%s\n", effectiveUser)

	script += fmt.Sprintf("echo \"Running placeholder...\"\n")
	script += fmt.Sprintf("echo \"SIF path: %s\"\n", shellQuote(resolvedSIFPath))
	script += fmt.Sprintf("singularity run %s\n", shellQuote(resolvedSIFPath))

	job := map[string]any{
		"name":                      fmt.Sprintf("gpu-switch-%s", req.ExecutionID),
		"environment":               map[string]string{"PATH": "/bin:/usr/bin:/usr/local/bin"},
		"current_working_directory": homeDir,
		"standard_output":           fmt.Sprintf("%s/gpu-switch-%s.out", homeDir, req.ExecutionID),
		"standard_error":            fmt.Sprintf("%s/gpu-switch-%s.err", homeDir, req.ExecutionID),
	}
	if req.Partition != "" {
		job["partition"] = req.Partition
	}
	if req.Account != "" {
		job["account"] = req.Account
	}
	body := map[string]any{
		"script": script,
		"job":    job,
	}

	resp, err := c.doRequestWithIdentity(ctx, http.MethodPost, "/slurm/v0.0.40/job/submit", marshalBody(body), "workload", effectiveUser, effectiveToken)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result jobSubmitResponse
	if err := c.decodeResponse(resp, &result); err != nil {
		return nil, err
	}

	if fatalErrs := filterFatalErrors(result.Errors); len(fatalErrs) > 0 {
		return nil, c.apiError(resp.StatusCode, fatalErrs)
	}

	return &PlaceholderJobResult{
		JobID: strconv.Itoa(result.JobID),
	}, nil
}

func (c *RestClient) GetNodeStateWithIdentity(ctx context.Context, nodeName string, id WorkloadIdentity) (*NodeState, error) {
	resp, err := c.doRequestWithIdentity(ctx, http.MethodGet, fmt.Sprintf("/slurm/v0.0.40/node/%s", nodeName), nil, "workload", id.User, id.Token)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return c.parseNodeStateResponse(resp)
}

func (c *RestClient) GetNodeState(ctx context.Context, nodeName string) (*NodeState, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, fmt.Sprintf("/slurm/v0.0.40/node/%s", nodeName), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return c.parseNodeStateResponse(resp)
}

func (c *RestClient) GetNodes(ctx context.Context) ([]NodeState, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/slurm/v0.0.40/nodes", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result nodeInfoResponse
	if err := c.decodeResponse(resp, &result); err != nil {
		return nil, err
	}

	if fatalErrs := filterFatalErrors(result.Errors); len(fatalErrs) > 0 {
		return nil, c.apiError(resp.StatusCode, fatalErrs)
	}

	var states []NodeState
	for _, node := range result.Nodes {
		state := NodeState{
			NodeName: node.Name,
			State:    strings.Join(node.State, "+"),
		}
		if node.Gres != "" {
			state.GRES = strings.Split(node.Gres, ",")
		}
		for _, jid := range node.AllocJobIDs {
			state.RunningJob = append(state.RunningJob, strconv.Itoa(jid))
		}
		states = append(states, state)
	}
	return states, nil
}

func (c *RestClient) parseNodeStateResponse(resp *http.Response) (*NodeState, error) {
	var result nodeInfoResponse
	if err := c.decodeResponse(resp, &result); err != nil {
		return nil, err
	}

	if fatalErrs := filterFatalErrors(result.Errors); len(fatalErrs) > 0 {
		return nil, c.apiError(resp.StatusCode, fatalErrs)
	}

	if len(result.Nodes) == 0 {
		return nil, &SlurmAPIError{StatusCode: 404, Messages: []string{"node not found"}}
	}

	node := result.Nodes[0]
	state := &NodeState{
		NodeName: node.Name,
		State:    strings.Join(node.State, "+"),
	}

	if node.Gres != "" {
		state.GRES = strings.Split(node.Gres, ",")
	}

	for _, jid := range node.AllocJobIDs {
		state.RunningJob = append(state.RunningJob, strconv.Itoa(jid))
	}

	return state, nil
}

func (c *RestClient) DrainNode(ctx context.Context, nodeName, reason string) error {
	body := map[string]any{
		"state":  []string{"DRAIN"},
		"reason": reason,
	}
	_, err := c.doJSONAdmin(ctx, http.MethodPost, fmt.Sprintf("/slurm/v0.0.40/node/%s", nodeName), body)
	if err != nil {
		if apiErr, ok := err.(*SlurmAPIError); ok && isDrainIdempotent(apiErr) {
			return nil
		}
		return err
	}
	return nil
}

func (c *RestClient) ResumeNode(ctx context.Context, nodeName string) error {
	body := map[string]any{
		"state": []string{"RESUME"},
	}
	_, err := c.doJSONAdmin(ctx, http.MethodPost, fmt.Sprintf("/slurm/v0.0.40/node/%s", nodeName), body)
	if err != nil {
		if apiErr, ok := err.(*SlurmAPIError); ok && isResumeIdempotent(apiErr) {
			return nil
		}
		return err
	}
	return nil
}

// isDrainIdempotent returns true when the API error indicates the node is already in drain/drained state.
func isDrainIdempotent(e *SlurmAPIError) bool {
	for _, msg := range e.Messages {
		l := strings.ToLower(msg)
		if strings.Contains(l, "already") && (strings.Contains(l, "drain") || strings.Contains(l, "drained")) {
			return true
		}
	}
	return false
}

// isResumeIdempotent returns true when the API error indicates the node is already in an active state.
func isResumeIdempotent(e *SlurmAPIError) bool {
	for _, msg := range e.Messages {
		l := strings.ToLower(msg)
		if strings.Contains(l, "already") && (strings.Contains(l, "resume") || strings.Contains(l, "idle") || strings.Contains(l, "active")) {
			return true
		}
	}
	return false
}

func (c *RestClient) VerifyToken(ctx context.Context, user, token string) error {
	resp, err := c.doRequestWithIdentity(ctx, http.MethodGet, "/slurm/v0.0.40/partitions", nil, "verify", user, token)
	if err != nil {
		return fmt.Errorf("slurm token verification failed: %w", err)
	}
	resp.Body.Close()
	return nil
}

func (c *RestClient) ListPartitions(ctx context.Context) ([]Partition, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, "/slurm/v0.0.40/partitions", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result partitionsResponse
	if err := c.decodeResponse(resp, &result); err != nil {
		return nil, err
	}

	if fatalErrs := filterFatalErrors(result.Errors); len(fatalErrs) > 0 {
		return nil, c.apiError(resp.StatusCode, fatalErrs)
	}

	var partitions []Partition
	for _, p := range result.Partitions {
		nodes := expandNodeList(p.Nodes.Configured)
		partitions = append(partitions, Partition{
			Name:  p.Name,
			Nodes: nodes,
		})
	}
	return partitions, nil
}

func (c *RestClient) CancelJob(ctx context.Context, jobID string) error {
	resp, err := c.doRequest(ctx, http.MethodDelete, fmt.Sprintf("/slurm/v0.0.40/job/%s", jobID), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result slurmErrorResponse
	if err := c.decodeResponse(resp, &result); err != nil {
		return err
	}

	if fatalErrs := filterFatalErrors(result.Errors); len(fatalErrs) > 0 {
		return c.apiError(resp.StatusCode, fatalErrs)
	}

	return nil
}

func (c *RestClient) CancelJobWithIdentity(ctx context.Context, jobID string, id WorkloadIdentity) error {
	resp, err := c.doRequestWithIdentity(ctx, http.MethodDelete, fmt.Sprintf("/slurm/v0.0.40/job/%s", jobID), nil, "workload", id.User, id.Token)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result slurmErrorResponse
	if err := c.decodeResponse(resp, &result); err != nil {
		return err
	}

	if fatalErrs := filterFatalErrors(result.Errors); len(fatalErrs) > 0 {
		return c.apiError(resp.StatusCode, fatalErrs)
	}

	return nil
}

func marshalBody(body any) io.Reader {
	data, _ := json.Marshal(body)
	return bytes.NewReader(data)
}

func (c *RestClient) doJSON(ctx context.Context, method, path string, body any) (*http.Response, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshaling request body: %w", err)
	}
	return c.doRequest(ctx, method, path, bytes.NewReader(data))
}

func (c *RestClient) doJSONAdmin(ctx context.Context, method, path string, body any) (*http.Response, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshaling request body: %w", err)
	}
	return c.doRequestWithIdentity(ctx, method, path, bytes.NewReader(data), "admin", c.adminUser, c.adminToken)
}

func (c *RestClient) doRequest(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	return c.doRequestWithIdentity(ctx, method, path, body, "admin", c.adminUser, c.adminToken)
}

func (c *RestClient) doRequestWithIdentity(ctx context.Context, method, path string, body io.Reader, identity, slurmUser, slurmToken string) (*http.Response, error) {
	started := time.Now()
	url := c.baseURL + path

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		c.logRequest(method, path, identity, started, 0, "", err)
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("X-SLURM-USER-NAME", slurmUser)
	req.Header.Set("X-SLURM-USER-TOKEN", slurmToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.logRequest(method, path, identity, started, 0, "", err)
		return nil, err
	}

	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		bodyBytes, err := io.ReadAll(resp.Body)
		if err != nil {
			apiErr := &SlurmAPIError{
				StatusCode: resp.StatusCode,
				Messages:   []string{http.StatusText(resp.StatusCode)},
			}
			c.logRequest(method, path, identity, started, resp.StatusCode, "", apiErr)
			return nil, apiErr
		}

		var errResp slurmErrorResponse
		if decErr := json.Unmarshal(bodyBytes, &errResp); decErr == nil && len(errResp.Errors) > 0 {
			fatalErrs := filterFatalErrors(errResp.Errors)
			if len(fatalErrs) == 0 {
				c.logger.Warn("slurmrestd returned non-fatal database errors; continuing with response data",
					"status_code", resp.StatusCode,
				)
				resp.Body = io.NopCloser(bytes.NewReader(bodyBytes))
				c.logRequest(method, path, identity, started, resp.StatusCode, "non-fatal: "+summarizeMessages(c.apiError(resp.StatusCode, errResp.Errors).Messages), nil)
				return resp, nil
			}

			apiErr := c.apiError(resp.StatusCode, errResp.Errors)
			c.logRequest(method, path, identity, started, resp.StatusCode, summarizeMessages(apiErr.Messages), apiErr)
			return nil, apiErr
		}

		apiErr := &SlurmAPIError{
			StatusCode: resp.StatusCode,
			Messages:   []string{http.StatusText(resp.StatusCode)},
		}
		c.logRequest(method, path, identity, started, resp.StatusCode, summarizeMessages(apiErr.Messages), apiErr)
		return nil, apiErr
	}

	c.logRequest(method, path, identity, started, resp.StatusCode, "", nil)

	return resp, nil
}

func (c *RestClient) decodeResponse(resp *http.Response, v any) error {
	return json.NewDecoder(resp.Body).Decode(v)
}

func (c *RestClient) apiError(statusCode int, errs []slurmError) *SlurmAPIError {
	msgs := make([]string, len(errs))
	for i, e := range errs {
		msgs[i] = e.Error
	}
	return &SlurmAPIError{StatusCode: statusCode, Messages: msgs}
}

func (c *RestClient) logRequest(method, path, identity string, started time.Time, statusCode int, apiErrorSummary string, err error) {
	attrs := []any{
		"event", "slurmrestd.request",
		"method", method,
		"path", path,
		"identity", identity,
		"latency", time.Since(started),
	}
	if statusCode > 0 {
		attrs = append(attrs, "status_code", statusCode)
	}
	if apiErrorSummary != "" {
		attrs = append(attrs, "api_error", apiErrorSummary)
	}
	if err != nil {
		attrs = append(attrs, "error", err.Error())
	}
	c.logger.Info("slurmrestd.request", attrs...)
}

func summarizeMessages(messages []string) string {
	return strings.Join(messages, "; ")
}

func isNonFatalError(err slurmError) bool {
	msg := strings.ToLower(err.Error)
	desc := strings.ToLower(err.Description)
	if strings.Contains(msg, "header lengths are longer than data received") ||
		strings.Contains(msg, "slurmdb query failed") ||
		strings.Contains(desc, "slurmdb query failed") ||
		strings.Contains(desc, "unable to query tres") ||
		err.Source == "slurmdb_tres_get" {
		return true
	}
	return false
}

func filterFatalErrors(errs []slurmError) []slurmError {
	var fatal []slurmError
	for _, e := range errs {
		if !isNonFatalError(e) {
			fatal = append(fatal, e)
		}
	}
	return fatal
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\\''") + "'"
}
