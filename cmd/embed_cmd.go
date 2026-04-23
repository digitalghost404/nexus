package cmd

import (
	"fmt"
	"net/http"
	"time"

	"github.com/digitalghost404/nexus/internal/db"
	"github.com/digitalghost404/nexus/internal/embed"
	"github.com/spf13/cobra"
)

var embedCmd = &cobra.Command{
	Use:   "embed",
	Short: "Manage embedding queue (backfill, re-embed, migrate model)",
	RunE: func(cmd *cobra.Command, args []string) error {
		backfill, _ := cmd.Flags().GetBool("backfill")
		reembed, _ := cmd.Flags().GetBool("reembed")
		migrateModel, _ := cmd.Flags().GetBool("migrate-model")

		if !backfill && !reembed && !migrateModel {
			_ = cmd.Usage()
			return nil
		}

		database, err := db.Open(cfg.DBPath())
		if err != nil {
			return err
		}
		defer func() { _ = database.Close() }()

		client := embed.NewClient(cfg.OllamaURL, cfg.OllamaModel, &http.Client{Timeout: 5 * time.Second})

		if migrateModel {
			return handleMigrateModel(database, client)
		}

		if reembed {
			return handleReembed(database, client)
		}

		if backfill {
			return handleBackfill(database, client, cmd)
		}

		return nil
	},
}

func handleBackfill(database *db.DB, client *embed.Client, cmd *cobra.Command) error {
	const batchSize = 50
	total := 0

	for {
		items, err := getUnembeddedItems(database, batchSize)
		if err != nil {
			return fmt.Errorf("get unembedded items: %w", err)
		}
		if len(items) == 0 {
			break
		}

		texts := make([]string, len(items))
		for i, item := range items {
			text, _ := embed.TruncateContent(item.content, 30000)
			texts[i] = text
		}

		vecs, err := client.EmbedBatch(cmd.Context(), texts)
		if err != nil {
			return fmt.Errorf("embed batch: %w", err)
		}

		for i, item := range items {
			if i >= len(vecs) {
				break
			}
			if err := storeEmbedding(database, item, vecs[i], client.Model()); err != nil {
				fmt.Printf("warning: store embedding for %s %d: %v\n", item.sourceType, item.sourceID, err)
			}
		}

		total += len(items)
		fmt.Printf("Embedded %d items (total: %d)\n", len(items), total)

		time.Sleep(2 * time.Second)
	}

	fmt.Printf("Backfill complete. %d items embedded.\n", total)
	return nil
}

func handleReembed(database *db.DB, client *embed.Client) error {
	model := client.Model()

	result, err := database.Conn().Exec("DELETE FROM embedding_meta WHERE embedding_model = ?", model)
	if err != nil {
		return fmt.Errorf("delete embedding_meta: %w", err)
	}
	deletedMeta, _ := result.RowsAffected()

	result, err = database.Conn().Exec(`
		DELETE FROM embeddings WHERE id NOT IN (SELECT id FROM embedding_meta)
	`)
	if err != nil {
		return fmt.Errorf("delete orphan embeddings: %w", err)
	}
	deletedVecs, _ := result.RowsAffected()

	fmt.Printf("Invalidated %d embedding meta records and %d orphan vectors for model %q.\n", deletedMeta, deletedVecs, model)
	fmt.Println("Run 'nexus embed --backfill' to re-embed.")
	return nil
}

func handleMigrateModel(database *db.DB, client *embed.Client) error {
	model := client.Model()

	result, err := database.Conn().Exec("DELETE FROM embeddings")
	if err != nil {
		return fmt.Errorf("delete all embeddings: %w", err)
	}
	deletedVecs, _ := result.RowsAffected()

	result, err = database.Conn().Exec("UPDATE embedding_meta SET status = 'pending', model_name = NULL")
	if err != nil {
		return fmt.Errorf("reset embedding meta: %w", err)
	}
	resetMeta, _ := result.RowsAffected()

	fmt.Printf("Reset %d embedding meta records and deleted %d vectors.\n", resetMeta, deletedVecs)
	fmt.Printf("Migrating to model %q. Starting backfill...\n", model)

	return handleBackfill(database, client, nil)
}

type unembeddedItem struct {
	sourceType string
	sourceID   int64
	content    string
}

func getUnembeddedItems(database *db.DB, limit int) ([]unembeddedItem, error) {
	query := `
		SELECT 'session' as source_type, s.id, s.summary as content
		FROM sessions s
		LEFT JOIN embedding_meta em ON em.source_type = 'session' AND em.source_id = s.id
		WHERE em.id IS NULL AND s.summary IS NOT NULL AND s.summary != ''
		UNION ALL
		SELECT 'note' as source_type, n.id, n.content
		FROM notes n
		LEFT JOIN embedding_meta em ON em.source_type = 'note' AND em.source_id = n.id
		WHERE em.id IS NULL AND n.content IS NOT NULL AND n.content != ''
		UNION ALL
		SELECT 'preference' as source_type, p.id, p.content
		FROM preferences p
		LEFT JOIN embedding_meta em ON em.source_type = 'preference' AND em.source_id = p.id
		WHERE em.id IS NULL AND p.content IS NOT NULL AND p.content != '' AND p.superseded_by IS NULL
		LIMIT ?
	`

	rows, err := database.Conn().Query(query, limit)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var items []unembeddedItem
	for rows.Next() {
		var item unembeddedItem
		if err := rows.Scan(&item.sourceType, &item.sourceID, &item.content); err != nil {
			continue
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func storeEmbedding(database *db.DB, item unembeddedItem, vec []float64, model string) error {
	return database.StoreEmbedding(item.sourceType, item.sourceID, item.content, vec, model, false)
}

func init() {
	embedCmd.GroupID = "maintenance"
	embedCmd.Flags().Bool("backfill", false, "Embed all unembedded items in batches")
	embedCmd.Flags().Bool("reembed", false, "Invalidate all vectors for current model")
	embedCmd.Flags().Bool("migrate-model", false, "Re-embed all vectors with current model")
	rootCmd.AddCommand(embedCmd)
}
