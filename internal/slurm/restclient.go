package slurm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"
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

type RestClient struct {
	baseURL            string
	jwtToken           string
	httpClient         *http.Client
	amqpURL            string
	placeholderSIFPath string
}

func NewRestClient(baseURL, jwtToken string, opts ...Option) *RestClient {
	rc := &RestClient{
		baseURL:  strings.TrimRight(baseURL, "/"),
		jwtToken: jwtToken,
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
	for _, o := range opts {
		o(rc)
	}
	return rc
}

func (c *RestClient) SubmitPlaceholderJob(ctx context.Context, req PlaceholderJobRequest) (*PlaceholderJobResult, error) {
	// For API v38 job submission, it's easier and more robust to use script directly.
	script := fmt.Sprintf("#!/bin/bash\n#SBATCH --job-name=gpu-switch-%s\n#SBATCH --nodes=1\n#SBATCH --ntasks=1\n#SBATCH --exclusive=user\n", req.ExecutionID)
	if req.Constraint != "" {
		script += fmt.Sprintf("#SBATCH --constraint=%s\n", req.Constraint)
	}
	if req.Partition != "" {
		script += fmt.Sprintf("#SBATCH --partition=%s\n", req.Partition)
	}

	// Environment vars
	script += fmt.Sprintf("export EXECUTION_ID=%s\n", req.ExecutionID)
	script += fmt.Sprintf("export AMQP_URL=%s\n", c.amqpURL)
	script += fmt.Sprintf("export SLURM_API_URL=%s\n", c.baseURL)
	script += fmt.Sprintf("export SLURM_JWT_TOKEN=%s\n", c.jwtToken)

	script += fmt.Sprintf("echo \"Running placeholder...\"\n")
	script += fmt.Sprintf("echo \"SIF path: %s\"\n", c.placeholderSIFPath)
	script += fmt.Sprintf("singularity run %s\n", c.placeholderSIFPath)

	body := map[string]any{
		"script": script,
		"job": map[string]any{
			"name":                      fmt.Sprintf("gpu-switch-%s", req.ExecutionID),
			"environment":               map[string]string{"PATH": "/bin:/usr/bin:/usr/local/bin"},
			"current_working_directory": "/tmp",
			"standard_output":           fmt.Sprintf("/tmp/gpu-switch-%s.out", req.ExecutionID),
			"standard_error":            fmt.Sprintf("/tmp/gpu-switch-%s.err", req.ExecutionID),
		},
	}

	resp, err := c.doJSON(ctx, http.MethodPost, "/slurm/v0.0.38/job/submit", body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result jobSubmitResponse
	if err := c.decodeResponse(resp, &result); err != nil {
		return nil, err
	}

	if len(result.Errors) > 0 {
		return nil, c.apiError(resp.StatusCode, result.Errors)
	}

	return &PlaceholderJobResult{
		JobID: strconv.Itoa(result.JobID),
	}, nil
}

func (c *RestClient) GetNodeState(ctx context.Context, nodeName string) (*NodeState, error) {
	resp, err := c.doRequest(ctx, http.MethodGet, fmt.Sprintf("/slurm/v0.0.38/node/%s", nodeName), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result nodeInfoResponse
	if err := c.decodeResponse(resp, &result); err != nil {
		return nil, err
	}

	if len(result.Errors) > 0 {
		return nil, c.apiError(resp.StatusCode, result.Errors)
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
	// Fallback to scontrol since REST API seems to lack node update in this version
	return fmt.Errorf("drain node via REST not supported in this version")
}

func (c *RestClient) ResumeNode(ctx context.Context, nodeName string) error {
	// Fallback to scontrol since REST API seems to lack node update in this version
	return fmt.Errorf("resume node via REST not supported in this version")
}

func (c *RestClient) CancelJob(ctx context.Context, jobID string) error {
	resp, err := c.doRequest(ctx, http.MethodDelete, fmt.Sprintf("/slurm/v0.0.38/job/%s", jobID), nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var result slurmErrorResponse
	if err := c.decodeResponse(resp, &result); err != nil {
		return err
	}

	if len(result.Errors) > 0 {
		return c.apiError(resp.StatusCode, result.Errors)
	}

	return nil
}

func (c *RestClient) doJSON(ctx context.Context, method, path string, body any) (*http.Response, error) {
	data, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshaling request body: %w", err)
	}
	return c.doRequest(ctx, method, path, bytes.NewReader(data))
}

func (c *RestClient) doRequest(ctx context.Context, method, path string, body io.Reader) (*http.Response, error) {
	url := c.baseURL + path

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return nil, fmt.Errorf("creating request: %w", err)
	}

	req.Header.Set("X-SLURM-USER-NAME", "cloud-user")
	req.Header.Set("X-SLURM-USER-TOKEN", c.jwtToken)
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		defer resp.Body.Close()
		var errResp slurmErrorResponse
		if decErr := json.NewDecoder(resp.Body).Decode(&errResp); decErr == nil && len(errResp.Errors) > 0 {
			return nil, c.apiError(resp.StatusCode, errResp.Errors)
		}
		return nil, &SlurmAPIError{
			StatusCode: resp.StatusCode,
			Messages:   []string{http.StatusText(resp.StatusCode)},
		}
	}

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
