package mcpserver

import (
	"context"
	_ "embed"
	"encoding/base64"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"github.com/igorrius/homelander-bot/internal/domain"
)

// Guide tells an agent how a Homelander page is built; page_guide returns it
// and it is the resource GuideURI.
//
//go:embed guide.md
var Guide string

// GuideURI is the MCP resource of the page guide.
const GuideURI = "homelander://guide/page"

// textExts are the page files whose content page_get_version returns as text.
var textExts = map[string]bool{".html": true, ".css": true, ".js": true, ".json": true, ".svg": true}

type (
	pageOut struct {
		Name string `json:"name"`
		Path string `json:"path" jsonschema:"the page's address on the site: / for index.html, /contacts for contacts.html"`
	}
	fileMeta struct {
		Path string `json:"path"`
		Size int    `json:"size"`
	}
	versionOut struct {
		ID         int64      `json:"id"`
		Status     string     `json:"status" jsonschema:"draft, published or archived"`
		Note       string     `json:"note"`
		CreatedAt  time.Time  `json:"created_at"`
		CreatedVia string     `json:"created_via" jsonschema:"the agent's client name when written through MCP"`
		Pages      []pageOut  `json:"pages" jsonschema:"root *.html files that are pages, with their site addresses"`
		Files      []fileMeta `json:"files"`
	}
	versionsOut struct {
		Versions []versionOut `json:"versions"`
	}
	idIn struct {
		ID int64 `json:"id"`
	}
	fileOut struct {
		Path        string `json:"path"`
		ContentType string `json:"content_type"`
		Size        int    `json:"size"`
		Text        string `json:"text,omitempty" jsonschema:"content of html, css, js, json and svg files"`
	}
	versionFilesOut struct {
		ID         int64     `json:"id"`
		Status     string    `json:"status"`
		Note       string    `json:"note"`
		CreatedVia string    `json:"created_via"`
		Pages      []pageOut `json:"pages"`
		Files      []fileOut `json:"files"`
	}
	fileIn struct {
		Path   string  `json:"path" jsonschema:"relative path such as index.html or img/logo.png"`
		Text   *string `json:"text,omitempty" jsonschema:"content of a text file"`
		Base64 *string `json:"base64,omitempty" jsonschema:"content of a binary file, standard base64"`
	}
	createDraftIn struct {
		Note  string   `json:"note,omitempty" jsonschema:"what this version changes, up to 200 characters"`
		Files []fileIn `json:"files" jsonschema:"every file of the version; index.html is required"`
	}
	updateDraftIn struct {
		ID     int64    `json:"id"`
		Put    []fileIn `json:"put,omitempty" jsonschema:"files to add or replace"`
		Delete []string `json:"delete,omitempty" jsonschema:"paths to remove"`
	}
	draftOut struct {
		ID       int64     `json:"id"`
		AdminURL string    `json:"admin_url" jsonschema:"where an admin previews and publishes the draft; give it to them"`
		Pages    []pageOut `json:"pages" jsonschema:"the draft's pages with their site addresses"`
	}
)

func pagesOf(files []domain.PageFile) []pageOut {
	out := []pageOut{}
	for _, p := range domain.PagesOf(files) {
		out = append(out, pageOut{p.Name, p.Path})
	}
	return out
}

func toVersion(v domain.PageVersion) versionOut {
	o := versionOut{ID: v.ID, Status: string(v.Status), Note: v.Note, CreatedAt: v.CreatedAt, CreatedVia: v.CreatedVia,
		Pages: pagesOf(v.Files), Files: make([]fileMeta, 0, len(v.Files))}
	for _, f := range v.Files {
		o.Files = append(o.Files, fileMeta{f.Path, f.Size})
	}
	return o
}

func toFileInputs(in []fileIn) ([]domain.PageFileInput, error) {
	out := make([]domain.PageFileInput, 0, len(in))
	for i, f := range in {
		if (f.Text == nil) == (f.Base64 == nil) {
			return nil, inputError("files[%d] (%s): give exactly one of text or base64", i, f.Path)
		}
		if f.Text != nil {
			out = append(out, domain.PageFileInput{Path: f.Path, Data: []byte(*f.Text)})
			continue
		}
		data, err := base64.StdEncoding.DecodeString(*f.Base64)
		if err != nil {
			return nil, inputError("files[%d] (%s): base64 is not valid standard base64", i, f.Path)
		}
		out = append(out, domain.PageFileInput{Path: f.Path, Data: data})
	}
	return out, nil
}

func (t tools) draft(v domain.PageVersion) draftOut {
	return draftOut{ID: v.ID, AdminURL: strings.TrimRight(t.d.PublicURL, "/") + "/admin/pages/" + strconv.FormatInt(v.ID, 10), Pages: pagesOf(v.Files)}
}

// addPages adds the page guide and the draft tools. There is deliberately no
// tool to publish, unpublish or delete a version: that is an admin's call.
func (t tools) addPages(s *mcp.Server) {
	d := t.d
	s.AddResource(&mcp.Resource{URI: GuideURI, Name: "page-guide", Title: "How to build the Homelander site", MIMEType: "text/markdown"},
		func(context.Context, *mcp.ReadResourceRequest) (*mcp.ReadResourceResult, error) {
			return &mcp.ReadResourceResult{Contents: []*mcp.ResourceContents{{URI: GuideURI, MIMEType: "text/markdown", Text: Guide}}}, nil
		})

	mcp.AddTool(s, &mcp.Tool{Name: "page_guide", Description: "Read this before writing the site: the sandbox rules, several pages and links between them, how to read public data, the height and language messages with ready snippets, and file limits."},
		func(context.Context, *mcp.CallToolRequest, noArgs) (*mcp.CallToolResult, any, error) {
			return &mcp.CallToolResult{Content: []mcp.Content{&mcp.TextContent{Text: Guide}}}, nil, nil
		})

	mcp.AddTool(s, &mcp.Tool{Name: "page_list_versions", Description: "List the home page versions, newest first: status (draft, published, archived), note, author agent and files."},
		handler(t, func(ctx context.Context, _ domain.Actor, _ noArgs) (versionsOut, error) {
			vs, err := d.Pages.List(ctx)
			out := versionsOut{Versions: make([]versionOut, 0, len(vs))}
			for _, v := range vs {
				out.Versions = append(out.Versions, toVersion(v))
			}
			return out, err
		}))

	mcp.AddTool(s, &mcp.Tool{Name: "page_get_version", Description: "Read one version: its files, with the content of text files (html, css, js, json, svg)."},
		handler(t, func(ctx context.Context, _ domain.Actor, in idIn) (versionFilesOut, error) {
			v, err := d.Pages.Get(ctx, in.ID)
			if err != nil {
				return versionFilesOut{}, err
			}
			files, err := d.Pages.Files(ctx, in.ID)
			if err != nil {
				return versionFilesOut{}, err
			}
			out := versionFilesOut{ID: v.ID, Status: string(v.Status), Note: v.Note, CreatedVia: v.CreatedVia, Pages: pagesOf(v.Files), Files: make([]fileOut, 0, len(files))}
			for _, f := range files {
				fo := fileOut{Path: f.Path, ContentType: f.ContentType, Size: f.Size}
				if textExts[path.Ext(f.Path)] {
					fo.Text = string(f.Data)
				}
				out.Files = append(out.Files, fo)
			}
			return out, nil
		}))

	mcp.AddTool(s, &mcp.Tool{Name: "page_create_draft", Description: "Create a new draft version from files (index.html required; every other root *.html is a page at /<name>). Returns the admin_url where an admin previews and publishes it; you cannot publish."},
		handler(t, func(ctx context.Context, a domain.Actor, in createDraftIn) (draftOut, error) {
			files, err := toFileInputs(in.Files)
			if err != nil {
				return draftOut{}, err
			}
			v, err := d.Pages.CreateDraft(ctx, files, in.Note, a)
			return t.draft(v), err
		}))

	mcp.AddTool(s, &mcp.Tool{Name: "page_update_draft", Description: "Add, replace (put) or remove (delete) files of a draft. Published and archived versions cannot change; copy them to a draft first."},
		handler(t, func(ctx context.Context, a domain.Actor, in updateDraftIn) (draftOut, error) {
			files, err := toFileInputs(in.Put)
			if err != nil {
				return draftOut{}, err
			}
			v, err := d.Pages.PutFiles(ctx, in.ID, files, in.Delete, a)
			return t.draft(v), err
		}))

	mcp.AddTool(s, &mcp.Tool{Name: "page_copy_to_draft", Description: "Start a new draft from any version (for example the published one) to change it."},
		handler(t, func(ctx context.Context, a domain.Actor, in idIn) (draftOut, error) {
			v, err := d.Pages.CopyToDraft(ctx, in.ID, a)
			return t.draft(v), err
		}))
}
