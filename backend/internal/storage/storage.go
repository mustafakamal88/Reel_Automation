package storage

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"
)

// VideoMetadata holds the per-video data written into each zip slot.
type VideoMetadata struct {
	Rank        int      `json:"rank"`
	Title       string   `json:"title"`
	Description string   `json:"description"`
	Hashtags    []string `json:"hashtags"`
	Platforms   []string `json:"platforms"`
	AIDisclosure bool    `json:"ai_disclosure"`
}

// BatchSummary is written as batch-summary.json at the zip root.
type BatchSummary struct {
	Date        string          `json:"date"`
	GeneratedAt time.Time       `json:"generated_at"`
	VideoCount  int             `json:"video_count"`
	Platforms   []string        `json:"platforms"`
	Videos      []VideoMetadata `json:"videos"`
}

// ComplianceChecklist is written as compliance-checklist.json at the zip root.
type ComplianceChecklist struct {
	Date                       string `json:"date"`
	AIGeneratedDisclosure      bool   `json:"ai_generated_disclosure"`
	NoImpersonation            bool   `json:"no_impersonation"`
	NoCopyrightedMusicUsed     bool   `json:"no_copyrighted_music_used"`
	NoFakeEngagement           bool   `json:"no_fake_engagement"`
	NoMedicalLegalFinancial    bool   `json:"no_medical_legal_financial_claims_without_review"`
	PaidPartnershipDeclared    bool   `json:"paid_partnership_declared"`
	HumanApprovalBeforePublish bool   `json:"human_approval_before_publish"`
}

// BuildBatchZip creates the ZIP package for a daily batch.
// videoDir is expected to contain subdirectories named video-01 through video-06,
// each with the required files. Returns the path to the created ZIP.
func BuildBatchZip(date, videoDir, outputDir string, summary BatchSummary, checklist ComplianceChecklist) (string, error) {
	zipName := fmt.Sprintf("trendcortex-daily-batch-%s.zip", date)
	zipPath := filepath.Join(outputDir, zipName)

	if err := os.MkdirAll(outputDir, 0750); err != nil {
		return "", fmt.Errorf("storage: mkdir output: %w", err)
	}

	f, err := os.Create(zipPath)
	if err != nil {
		return "", fmt.Errorf("storage: create zip: %w", err)
	}
	defer f.Close()

	zw := zip.NewWriter(f)
	defer zw.Close()

	// Add all video slot directories
	for i := 1; i <= 6; i++ {
		slotDir := filepath.Join(videoDir, fmt.Sprintf("video-%02d", i))
		if err := addDirToZip(zw, slotDir, fmt.Sprintf("video-%02d/", i)); err != nil {
			return "", fmt.Errorf("storage: add video-%02d: %w", i, err)
		}
	}

	// Write batch-summary.json
	if err := writeJSONToZip(zw, "batch-summary.json", summary); err != nil {
		return "", fmt.Errorf("storage: write batch-summary.json: %w", err)
	}

	// Write compliance-checklist.json
	if err := writeJSONToZip(zw, "compliance-checklist.json", checklist); err != nil {
		return "", fmt.Errorf("storage: write compliance-checklist.json: %w", err)
	}

	return zipPath, nil
}

func addDirToZip(zw *zip.Writer, srcDir, prefix string) error {
	entries, err := os.ReadDir(srcDir)
	if os.IsNotExist(err) {
		// Slot directory not yet created (rendering not complete) — skip gracefully.
		return nil
	}
	if err != nil {
		return err
	}
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		srcPath := filepath.Join(srcDir, e.Name())
		fw, err := zw.Create(prefix + e.Name())
		if err != nil {
			return err
		}
		src, err := os.Open(srcPath)
		if err != nil {
			return err
		}
		if _, err := io.Copy(fw, src); err != nil {
			src.Close()
			return err
		}
		src.Close()
	}
	return nil
}

func writeJSONToZip(zw *zip.Writer, name string, v any) error {
	fw, err := zw.Create(name)
	if err != nil {
		return err
	}
	enc := json.NewEncoder(fw)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}
