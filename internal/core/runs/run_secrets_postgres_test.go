package runs

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/marmotdata/marmot/internal/plugin"
	"github.com/marmotdata/marmot/internal/store/postgres/pgtest"
)

func TestMaskStoredSecrets_MasksRunsStoredInPlaintext(t *testing.T) {
	const known = "runsecrets-stored"
	registerSecretPlugin(t, known)
	pool := pgtest.TempDB(t)
	ctx := context.Background()
	insert := func(source, config string) string {
		var id string
		require.NoError(t, pool.QueryRow(ctx, `
			INSERT INTO runs (id, pipeline_name, source_name, run_id, status, started_at, config, created_by)
			VALUES (gen_random_uuid(), 'p', $1, gen_random_uuid()::text, 'completed', now(), $2::jsonb, 'test')
			RETURNING id::text`, source, config).Scan(&id))
		return id
	}
	plaintext := insert(known, `{"host":"db.internal","password":"hunter2"}`)
	unknown := insert("runsecrets-not-loaded", `{"password":"hunter2"}`)
	repo := NewPostgresRepository(pool)

	n, err := repo.MaskStoredSecrets(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, n)

	stored := func(id string) map[string]interface{} {
		var config map[string]interface{}
		require.NoError(t, pool.QueryRow(ctx, `SELECT config FROM runs WHERE id = $1::uuid`, id).Scan(&config))
		return config
	}
	assert.Equal(t, map[string]interface{}{"host": "db.internal", "password": plugin.SensitiveMask}, stored(plaintext))
	assert.Equal(t, "hunter2", stored(unknown)["password"], "without the plugin's spec nothing says which field is secret")

	n, err = repo.MaskStoredSecrets(ctx)
	require.NoError(t, err)
	assert.Zero(t, n)
}
