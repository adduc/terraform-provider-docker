package internal

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"

	"github.com/hashicorp/terraform-plugin-framework/attr"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/path"
	"github.com/hashicorp/terraform-plugin-framework/types"
	"github.com/moby/moby/api/pkg/stdcopy"
	"github.com/moby/moby/client"
)

func NewLogsDataSource() datasource.DataSource {
	return &LogsDataSource{}
}

type LogsDataSource struct {
	DockerClient *client.Client
}

type LogsDataSourceModel struct {
	Container  types.String     `tfsdk:"container"`
	Logs       types.List       `tfsdk:"logs"`
	Timestamps types.Bool       `tfsdk:"timestamps"`
	Bypass     *LogsBypassModel `tfsdk:"bypass"`
}

type LogsBypassModel struct {
	TTYTimestamps types.Bool `tfsdk:"tty_timestamps"`
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
				Description: "Whether to include the timestamp of each log line. Defaults to true. An error for containers with a TTY unless bypass.tty_timestamps is set",
			},

			"bypass": schema.SingleNestedAttribute{
				Optional:    true,
				Description: "Disable safeguards that stop known-incorrect results",
				Attributes: map[string]schema.Attribute{
					"tty_timestamps": schema.BoolAttribute{
						Optional:    true,
						Description: "Allow timestamps for a container with a TTY. Docker splits lines over 16 KiB into parts, and in a TTY container's raw output the timestamps of later parts can't be told apart from the message, so they appear inside it",
					},
				},
			},

			// Computed

			"logs": schema.ListNestedAttribute{
				Computed:    true,
				Description: "The logs of the container",
				NestedObject: schema.NestedAttributeObject{
					Attributes: map[string]schema.Attribute{
						"stdout": schema.BoolAttribute{
							Computed:    true,
							Description: "Whether the log is from stdout",
						},
						"stderr": schema.BoolAttribute{
							Computed:    true,
							Description: "Whether the log is from stderr",
						},
						"message": schema.StringAttribute{
							Computed:    true,
							Description: "The log message",
						},
						"timestamp": schema.StringAttribute{
							Computed:    true,
							Description: "The log timestamp in RFC3339Nano format, or null when timestamps is false",
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

	// TTY containers write raw output, while others multiplex stdout and
	// stderr into framed messages, so the stream format depends on the
	// container.

	inspect, err := d.DockerClient.ContainerInspect(ctx, data.Container.ValueString(), client.ContainerInspectOptions{})

	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Inspect Container",
			fmt.Sprintf("Error inspecting container %q: %v", data.Container.ValueString(), err),
		)
		return
	}

	tty := inspect.Container.Config != nil && inspect.Container.Config.Tty

	if tty && data.Timestamps.ValueBool() && (data.Bypass == nil || !data.Bypass.TTYTimestamps.ValueBool()) {
		resp.Diagnostics.AddAttributeError(
			path.Root("timestamps"),
			"Timestamps Unsupported for TTY Containers",
			fmt.Sprintf("Container %q has a TTY, so log lines over 16 KiB would contain stray timestamps. "+
				"Set timestamps = false, or set bypass = { tty_timestamps = true } to read them anyway.", data.Container.ValueString()),
		)
		return
	}

	// get container logs

	options := client.ContainerLogsOptions{
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

	entries, err := readLogs(logs, tty, options.Timestamps)

	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Read Container Logs",
			fmt.Sprintf("Error reading logs for container %q: %v", data.Container.ValueString(), err),
		)
		return
	}

	// set logs

	logLines := make([]attr.Value, 0, len(entries))
	for _, entry := range entries {
		timestamp := types.StringNull()
		if options.Timestamps {
			timestamp = types.StringValue(entry.Timestamp)
		}

		logLines = append(logLines, types.ObjectValueMust(
			logEntryAttrTypes,
			map[string]attr.Value{
				"stdout":    types.BoolValue(entry.Stream == stdcopy.Stdout),
				"stderr":    types.BoolValue(entry.Stream == stdcopy.Stderr),
				"message":   types.StringValue(entry.Message),
				"timestamp": timestamp,
			},
		))
	}

	data.Logs = types.ListValueMust(types.ObjectType{AttrTypes: logEntryAttrTypes}, logLines)

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}

var logEntryAttrTypes = map[string]attr.Type{
	"stdout":    types.BoolType,
	"stderr":    types.BoolType,
	"message":   types.StringType,
	"timestamp": types.StringType,
}

// logEntry is one line of container output.
type logEntry struct {
	Stream    stdcopy.StdType
	Timestamp string // empty when timestamps are disabled
	Message   string
}

// readLogs parses a container log stream into lines.
//
// A non-TTY stream is a sequence of frames, each an 8-byte header (stream
// type and payload length) followed by one log message; stdcopy.StdCopy
// splits these by stream. A TTY stream is raw stdout with no framing.
//
// When timestamps are enabled, the daemon prefixes every message with a
// timestamp and a space. Lines longer than the daemon's buffer are split into
// several messages, each with its own timestamp, which are joined back
// together here.
func readLogs(r io.Reader, tty, timestamps bool) ([]logEntry, error) {
	p := &logParser{timestamps: timestamps, open: map[stdcopy.StdType]int{}}

	if tty {
		br := bufio.NewReader(r)
		for {
			line, err := br.ReadString('\n')
			if line != "" {
				if perr := p.add(stdcopy.Stdout, line); perr != nil {
					return nil, perr
				}
			}
			if err == io.EOF {
				return p.entries, nil
			}
			if err != nil {
				return nil, err
			}
		}
	}

	if _, err := stdcopy.StdCopy(p.writer(stdcopy.Stdout), p.writer(stdcopy.Stderr), r); err != nil {
		return nil, err
	}
	return p.entries, nil
}

type logParser struct {
	timestamps bool
	entries    []logEntry
	// open holds, per stream, the index of the entry whose line has not yet
	// been terminated by a newline.
	open map[stdcopy.StdType]int
}

// add records one log message from stream. The message may contain several
// lines, and may end without a newline when the line continues in the next
// message.
func (p *logParser) add(stream stdcopy.StdType, msg string) error {
	var timestamp string
	if p.timestamps {
		var ok bool
		if timestamp, msg, ok = strings.Cut(msg, " "); !ok {
			return fmt.Errorf("log message has no timestamp: %q", timestamp)
		}
	}

	for msg != "" {
		line, rest, complete := strings.Cut(msg, "\n")

		i, ok := p.open[stream]
		if ok {
			p.entries[i].Message += line
		} else {
			p.entries = append(p.entries, logEntry{Stream: stream, Timestamp: timestamp, Message: line})
			i = len(p.entries) - 1
		}

		if !complete {
			p.open[stream] = i
			break
		}

		// TTY output ends lines with \r\n.
		p.entries[i].Message = strings.TrimSuffix(p.entries[i].Message, "\r")
		delete(p.open, stream)
		msg = rest
	}

	return nil
}

// writer returns an io.Writer that records each Write as one message from
// stream. stdcopy.StdCopy writes each frame's payload in a single call.
func (p *logParser) writer(stream stdcopy.StdType) io.Writer {
	return logWriter(func(b []byte) error { return p.add(stream, string(b)) })
}

type logWriter func([]byte) error

func (w logWriter) Write(b []byte) (int, error) {
	if err := w(b); err != nil {
		return 0, err
	}
	return len(b), nil
}
