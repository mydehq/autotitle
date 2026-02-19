// Package tagger embeds metadata into MKV files using mkvpropedit.
package tagger

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"
)

const binaryName = "mkvpropedit"

// TagInfo contains the metadata to embed into an MKV file.
type TagInfo struct {
	Title       string // Episode title (goes into segment info)
	Show        string // Series name (Matroska tag: SHOW)
	EpisodeID   string // Formatted episode number (e.g. "01")
	EpisodeSort int    // Numeric episode number for sorting (PART_NUMBER)
	AirDate     string // ISO date string (e.g. "2013-04-07"), optional (DATE_RELEASED)
}

// IsAvailable returns true if mkvpropedit is found in $PATH.
func IsAvailable() bool {
	_, err := exec.LookPath(binaryName)
	return err == nil
}

// TagFile embeds metadata into a single MKV file using mkvpropedit.
// Non-MKV files are silently skipped (returns nil).
func TagFile(ctx context.Context, path string, info TagInfo) error {
	if !isMKV(path) {
		return nil
	}

	// Write tags via a temporary XML file (required by mkvpropedit --tags flag)
	tmpFile, err := os.CreateTemp("", "autotitle-tags-*.xml")
	if err != nil {
		return fmt.Errorf("failed to create temp tag file: %w", err)
	}
	defer os.Remove(tmpFile.Name())

	if err := writeTagXML(tmpFile, info); err != nil {
		tmpFile.Close()
		return fmt.Errorf("failed to write tag XML: %w", err)
	}
	tmpFile.Close()

	args := []string{
		path,
		// Set the segment title (appears as file title in media players)
		"--edit", "info",
		"--set", fmt.Sprintf("title=%s", info.Title),
		// Inject global Matroska tags from XML
		"--tags", fmt.Sprintf("all:%s", tmpFile.Name()),
	}

	cmd := exec.CommandContext(ctx, binaryName, args...)
	if out, err := cmd.CombinedOutput(); err != nil {
		return fmt.Errorf("mkvpropedit failed: %w\noutput: %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// isMKV returns true if the file has an .mkv extension.
func isMKV(path string) bool {
	return strings.EqualFold(filepath.Ext(path), ".mkv")
}

// tagXMLTemplate is the Matroska global tag XML format.
const tagXMLTemplate = `<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE Tags SYSTEM "matroskatags.dtd">
<Tags>
  <Tag>
    <Targets>
      <TargetTypeValue>50</TargetTypeValue>
      <TargetType>SHOW</TargetType>
    </Targets>
    <Simple>
      <Name>TITLE</Name>
      <String>{{.Show}}</String>
    </Simple>
  </Tag>
  <Tag>
    <Targets>
      <TargetTypeValue>30</TargetTypeValue>
      <TargetType>CHAPTER</TargetType>
    </Targets>
    <Simple>
      <Name>TITLE</Name>
      <String>{{.Title}}</String>
    </Simple>{{if .EpisodeID}}
    <Simple>
      <Name>PART_NUMBER</Name>
      <String>{{.EpisodeID}}</String>
    </Simple>{{end}}{{if .AirDate}}
    <Simple>
      <Name>DATE_RELEASED</Name>
      <String>{{.AirDate}}</String>
    </Simple>{{end}}
  </Tag>
</Tags>
`

var tagTmpl = template.Must(template.New("tags").Parse(tagXMLTemplate))

func writeTagXML(f *os.File, info TagInfo) error {
	return tagTmpl.Execute(f, info)
}
