package internal

import (
	"bufio"
	"context"
	"fmt"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/hashicorp/terraform-plugin-framework/types/basetypes"
)

// Docker log format constants
const (
	// DockerLogHeaderSize is the size of the Docker log entry header
	DockerLogHeaderSize = 8
	// DockerLogTimestampSize is the size of the RFC3339 timestamp in Docker logs
	DockerLogTimestampSize = 22
	// DockerLogTimestampEnd is the position where the timestamp ends
	DockerLogTimestampEnd = DockerLogHeaderSize + DockerLogTimestampSize // 30
	// DockerLogMessageStart is the position where the log message starts (after timestamp + separator)
	DockerLogMessageStart = DockerLogTimestampEnd + 9 // 39
)

func NewLogsDataSource() datasource.DataSource {
	return &LogsDataSource{}
}

type LogsDataSource struct {
	DockerClient *client.Client
}

type LogsDataSourceModel struct {
	Container  types.String `tfsdk:"container"`
	Logs       types.List   `tfsdk:"logs"`
	Timestamps types.Bool   `tfsdk:"timestamps"`
}

func (d *LogsDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_logs"
}

func (d *LogsDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{

			// Required

			"container": schema.StringAttribute{
				Required:    true,
				Description: "The name of the container",
			},

			// Optional

			"timestamps": schema.BoolAttribute{
				Optional:    true,
				Description: "Whether the log has timestamps",
			},

			// Computed

			"logs": schema.ListNestedAttribute{
				Computed:    true,
				Description: "The logs of the container",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"stdout": schema.BoolAttribute{
							Required:    true,
							Description: "Whether the log is from stdout",
						},
						"stderr": schema.BoolAttribute{
							Required:    true,
							Description: "Whether the log is from stderr",
						},
						"message": schema.StringAttribute{
							Required:    true,
							Description: "The log message",
						},
						"timestamp": schema.StringAttribute{
							Required:    true,
							Description: "The log timestamp",
						},
					},
				},
			},
		},
	}
}

func (d *LogsDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
	if req.ProviderData == nil {
		return
	}

	config, ok := req.ProviderData.(ProviderConfig)

	if !ok {
		resp.Diagnostics.AddError(
			"Unexpected Data Source Configure Type",
			fmt.Sprintf("Expected *ProviderConfig, got: %T. Please report this issue to the provider developers.", req.ProviderData),
		)
		return
	}

	d.DockerClient = config.DockerClient
}

func (d *LogsDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data LogsDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// timestamps defaults to true
	if data.Timestamps.IsNull() {
		data.Timestamps = types.BoolValue(true)
	}

	// Validate container name
	if err := validateContainerName(data.Container.ValueString()); err != nil {
		resp.Diagnostics.AddError(
			"Invalid Container Name",
			fmt.Sprintf("Container name validation failed: %v", err),
		)
		return
	}

	// get container logs

	options := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Timestamps: data.Timestamps.ValueBool(),
	}

	logs, err := d.DockerClient.ContainerLogs(ctx, data.Container.ValueString(), options)

	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Read Container Logs",
			fmt.Sprintf("Error reading logs for container %q: %v", data.Container.ValueString(), err),
		)
		return
	}
	defer func() {
		if closeErr := logs.Close(); closeErr != nil {
			resp.Diagnostics.AddWarning(
				"Resource Cleanup Warning",
				fmt.Sprintf("Failed to close log stream for container %q: %v", data.Container.ValueString(), closeErr),
			)
		}
	}()

	// parse logs

	var logLines []attr.Value
	scanner := bufio.NewScanner(logs)

	for scanner.Scan() {
		line := scanner.Text()
		logLine, err := processLogLine(line, options)
		if err != nil {
			resp.Diagnostics.AddError(
				"Unable to Process Log Line",
				fmt.Sprintf("Error processing log line for container %q: %v", data.Container.ValueString(), err),
			)
			return
		}
		logLines = append(logLines, logLine)
	}

	if err := scanner.Err(); err != nil {
		resp.Diagnostics.AddError(
			"Unable to Read Container Logs",
			fmt.Sprintf("Error reading logs for container %q: %v", data.Container.ValueString(), err),
		)
		return
	}

	// set logs

	data.Logs = types.ListValueMust(
		types.ObjectType{
			AttrTypes: map[string]attr.Type{
				"stdout":    types.BoolType,
				"stderr":    types.BoolType,
				"message":   types.StringType,
				"timestamp": types.StringType,
			},
		},
		logLines,
	)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

func processLogLine(line string, logOptions container.LogsOptions) (attr.Value, error) {
	// first byte in line is the stream type
	// 0: stdin
	// 1: stdout
	// 2: stderr
	if len(line) == 0 {
		return nil, fmt.Errorf("empty log line")
	}
	
	streamType := line[0]
	stdout, stderr := false, false

	switch streamType {
	case '\x00': // stdin
		return nil, fmt.Errorf("unexpected stdin log line")
	case '\x01': // stdout
		stdout = true
	case '\x02': // stderr
		stderr = true
	default:
		return nil, fmt.Errorf("unknown log line type: %q", streamType)
	}

	var timestamp, message basetypes.StringValue

	if logOptions.Timestamps {
		if len(line) < DockerLogMessageStart {
			return nil, fmt.Errorf("log line too short for timestamp parsing: need at least %d characters, got %d", DockerLogMessageStart, len(line))
		}
		timestamp = types.StringValue(line[DockerLogHeaderSize:DockerLogTimestampEnd])
		message = types.StringValue(line[DockerLogMessageStart:])
	} else {
		if len(line) < DockerLogHeaderSize {
			return nil, fmt.Errorf("log line too short: need at least %d characters, got %d", DockerLogHeaderSize, len(line))
		}
		message = types.StringValue(line[DockerLogHeaderSize:])
	}

	return types.ObjectValueMust(
		map[string]attr.Type{
			"stdout":    types.BoolType,
			"stderr":    types.BoolType,
			"message":   types.StringType,
			"timestamp": types.StringType,
		},

		map[string]attr.Value{
			"stdout":    types.BoolValue(stdout),
			"stderr":    types.BoolValue(stderr),
			"message":   message,
			"timestamp": timestamp,
		},
	), nil
}
