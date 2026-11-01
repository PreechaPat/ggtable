package sqldb

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"github.com/yumyai/ggtable/pkg/model"
)

// ErrInvalidSearch is wrapped by search errors caused by the request itself
// (unknown search or sort field), as opposed to database failures.
var ErrInvalidSearch = fmt.Errorf("invalid search")

// MainPage returns one page of all clusters, ordered by cluster ID.
func (db *DB) MainPage(ctx context.Context, req model.ClusterSearchRequest) ([]*model.Cluster, error) {
	const q = `
		CREATE TEMPORARY TABLE unique_clusters AS
		SELECT cluster_id, cog_id, expected_length, function_description, representative_gene
		FROM gene_clusters
		ORDER BY cluster_id
		LIMIT ? OFFSET ?`
	return db.page(ctx, 10*time.Second, req, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, q, req.Page_Size, req.Offset()); err != nil {
			return fmt.Errorf("select main page clusters: %w", err)
		}
		return nil
	})
}

// Search returns one page of clusters matching req: by exact gene ID for
// ClusterFieldGeneID, otherwise by substring of the chosen cluster field.
func (db *DB) Search(ctx context.Context, req model.ClusterSearchRequest) ([]*model.Cluster, error) {
	orderBy, err := orderByExpr(req.Order_By)
	if err != nil {
		return nil, err
	}
	if req.Search_Field == model.ClusterFieldGeneID {
		q := `
			CREATE TEMPORARY TABLE unique_clusters AS
			SELECT gc.cluster_id, gc.cog_id, gc.expected_length, gc.function_description, gc.representative_gene
			FROM gene_clusters gc
			WHERE gc.cluster_id IN (
				SELECT gm.cluster_id FROM gene_matches gm
				WHERE gm.gene_id = ? AND ` + genomeFilter("gm.genome_id") + `)
			ORDER BY ` + orderBy + `
			LIMIT ? OFFSET ?`
		return db.page(ctx, 20*time.Second, req, func(tx *sql.Tx) error {
			if _, err := tx.ExecContext(ctx, q, req.Search_For, req.Page_Size, req.Offset()); err != nil {
				return fmt.Errorf("select clusters by gene: %w", err)
			}
			return nil
		})
	}

	where, err := whereExpr(req.Search_Field)
	if err != nil {
		return nil, err
	}
	q := `
		CREATE TEMPORARY TABLE unique_clusters AS
		SELECT gc.cluster_id, gc.cog_id, gc.expected_length, gc.function_description, gc.representative_gene
		FROM gene_clusters gc
		WHERE ` + where + `
		ORDER BY ` + orderBy + `
		LIMIT ? OFFSET ?`
	return db.page(ctx, 20*time.Second, req, func(tx *sql.Tx) error {
		if _, err := tx.ExecContext(ctx, q, likePattern(req.Search_For), req.Page_Size, req.Offset()); err != nil {
			return fmt.Errorf("select clusters by %s: %w", req.Search_Field, err)
		}
		return nil
	})
}

// CountSearch returns how many clusters Search would return over all pages.
func (db *DB) CountSearch(ctx context.Context, req model.ClusterSearchRequest) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var count int
	err := db.withReadTx(ctx, func(tx *sql.Tx) error {
		if req.Search_Field == model.ClusterFieldGeneID {
			if err := createGenomeFilter(ctx, tx, req.Genome_IDs); err != nil {
				return err
			}
			q := `SELECT COUNT(DISTINCT gm.cluster_id) FROM gene_matches gm
			      WHERE gm.gene_id = ? AND ` + genomeFilter("gm.genome_id")
			return tx.QueryRowContext(ctx, q, req.Search_For).Scan(&count)
		}
		where, err := whereExpr(req.Search_Field)
		if err != nil {
			return err
		}
		q := `SELECT COUNT(*) FROM gene_clusters gc WHERE ` + where
		return tx.QueryRowContext(ctx, q, likePattern(req.Search_For)).Scan(&count)
	})
	if err != nil {
		return 0, fmt.Errorf("count search results: %w", err)
	}
	return count, nil
}

// CountClusters returns the total number of clusters.
func (db *DB) CountClusters(ctx context.Context) (int, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	var count int
	if err := db.sql.QueryRowContext(ctx, `SELECT COUNT(*) FROM gene_clusters`).Scan(&count); err != nil {
		return 0, fmt.Errorf("count clusters: %w", err)
	}
	return count, nil
}

// page runs scaffold (which must create unique_clusters) with the request's
// genome filter in place, then hydrates the result.
func (db *DB) page(ctx context.Context, timeout time.Duration, req model.ClusterSearchRequest, scaffold func(*sql.Tx) error) ([]*model.Cluster, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	var clusters []*model.Cluster
	err := db.withReadTx(ctx, func(tx *sql.Tx) error {
		if err := createGenomeFilter(ctx, tx, req.Genome_IDs); err != nil {
			return err
		}
		if err := scaffold(tx); err != nil {
			return err
		}
		var err error
		clusters, err = hydrate(ctx, tx)
		return err
	})
	if err != nil {
		return nil, err
	}
	return clusters, nil
}

func likePattern(term string) string { return "%" + term + "%" }

// whereExpr returns the WHERE condition for a substring search on field.
func whereExpr(field model.ClusterField) (string, error) {
	switch field {
	case model.ClusterFieldFunction:
		return "gc.function_description LIKE ?", nil
	case model.ClusterFieldCOGID:
		return "gc.cog_id LIKE ?", nil
	case model.ClusterFieldClusterID:
		return "gc.cluster_id LIKE ?", nil
	default:
		return "", fmt.Errorf("%w: cannot search by %q, use function, cog_id, cluster_id or gene_id", ErrInvalidSearch, field)
	}
}

// orderByExpr returns the ORDER BY expression for field.
func orderByExpr(field model.ClusterField) (string, error) {
	switch field {
	case model.ClusterFieldFunction:
		return "gc.function_description", nil
	case model.ClusterFieldCOGID:
		return "gc.cog_id", nil
	case model.ClusterFieldClusterID:
		return "gc.cluster_id", nil
	default:
		return "", fmt.Errorf("%w: cannot sort by %q, use function, cog_id or cluster_id", ErrInvalidSearch, field)
	}
}
