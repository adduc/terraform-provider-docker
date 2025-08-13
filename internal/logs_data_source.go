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

func NewLogsDataSource() datasource.DataSource {
	return &LogsDataSource{}
}

type LogsDataSource struct {
	DockerClient *client.Client
}

type LogsDataSourceModel struct {
	Name       types.String `tfsdk:"name"`
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

			"name": schema.StringAttribute{
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

	// get container logs

	options := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Timestamps: data.Timestamps.ValueBool(),
	}

	logs, err := d.DockerClient.ContainerLogs(ctx, data.Name.ValueString(), options)

	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Read Container Logs",
			fmt.Sprintf("Error reading logs for container %q: %v", data.Name.ValueString(), err),
		)
		return
	}
	defer logs.Close()

	// parse logs

	var logLines []attr.Value
	scanner := bufio.NewScanner(logs)

	for scanner.Scan() {
		line := scanner.Text()
		logLines = append(logLines, processLogLine(line, options))
	}

	if err := scanner.Err(); err != nil {
		resp.Diagnostics.AddError(
			"Unable to Read Container Logs",
			fmt.Sprintf("Error reading logs for container %q: %v", data.Name.ValueString(), err),
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

func processLogLine(line string, logOptions container.LogsOptions) attr.Value {
	// first byte in line is the stream type
	// 0: stdin
	// 1: stdout
	// 2: stderr
	streamType := line[0]

	stdout, stderr := false, false

	switch streamType {
	case '\x00': // stdin
		panic("stdin log line?")
	case '\x01': // stdout
		stdout = true
	case '\x02': // stderr
		stderr = true
	default:
		panic(fmt.Sprintf("unknown log line type: %q", streamType))
	}

	var timestamp, message basetypes.StringValue

	if logOptions.Timestamps {
		timestamp = types.StringValue(line[8:30])
		message = types.StringValue(line[39:])
	} else {
		message = types.StringValue(line[8:])
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
	)
}
