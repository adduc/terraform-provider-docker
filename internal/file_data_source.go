package internal

import (
	"context"
	"fmt"
	"io"

	"github.com/docker/docker/client"
	"github.com/hashicorp/terraform-plugin-framework/datasource"
	"github.com/hashicorp/terraform-plugin-framework/datasource/schema"
	"github.com/hashicorp/terraform-plugin-framework/types"
)

func NewFileDataSource() datasource.DataSource {
	return &FileDataSource{}
}

type FileDataSource struct {
	DockerClient *client.Client
}

type FileDataSourceModel struct {
	Container types.String `tfsdk:"container"`
	Path      types.String `tfsdk:"path"`
	Content   types.String `tfsdk:"content"`
}

func (d *FileDataSource) Metadata(ctx context.Context, req datasource.MetadataRequest, resp *datasource.MetadataResponse) {
	resp.TypeName = req.ProviderTypeName + "_file"
}

func (d *FileDataSource) Schema(ctx context.Context, req datasource.SchemaRequest, resp *datasource.SchemaResponse) {
	resp.Schema = schema.Schema{
		Attributes: map[string]schema.Attribute{

			// Required

			"container": schema.StringAttribute{
				Required:    true,
				Description: "The name of the container",
			},

			"path": schema.StringAttribute{
				Required:    true,
				Description: "The path to the file to retrieve from the container",
			},

			// Computed

			"content": schema.StringAttribute{
				Computed:    true,
				Description: "The content of the file",
				Sensitive:   true,
			},
		},
	}
}

func (d *FileDataSource) Configure(ctx context.Context, req datasource.ConfigureRequest, resp *datasource.ConfigureResponse) {
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

func (d *FileDataSource) Read(ctx context.Context, req datasource.ReadRequest, resp *datasource.ReadResponse) {
	var data FileDataSourceModel

	resp.Diagnostics.Append(req.Config.Get(ctx, &data)...)

	if resp.Diagnostics.HasError() {
		return
	}

	// @todo fetch file content

	// $ curl --unix-socket /var/run/docker.sock http://localhost/containers/992906944bee/archive --url-query "path=/etc/ssl/certs/ca-certificates.crt" --output -

	// @todo read file stat info into computed attribute(s)
	file, _, err := d.DockerClient.CopyFromContainer(ctx, data.Container.ValueString(), data.Path.ValueString())

	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Read File from Container",
			fmt.Sprintf("Error reading file %q from container %q: %v", data.Path.ValueString(), data.Container.ValueString(), err),
		)
		return
	}
	defer file.Close()

	content, err := io.ReadAll(file)

	if err != nil {
		resp.Diagnostics.AddError(
			"Unable to Read File Content",
			fmt.Sprintf("Error reading file content from container %q: %v", data.Container.ValueString(), err),
		)
		return
	}

	data.Content = types.StringValue(string(content))

	resp.Diagnostics.Append(resp.State.Set(ctx, &data)...)
}
