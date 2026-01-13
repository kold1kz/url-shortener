package repository

import (
	"context"
	"database/sql"
	"errors"
	"regexp"
	"testing"
	"url-shortener/internal/model"

	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/jackc/pgerrcode"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/stretchr/testify/require"
)

func newMockDB(t *testing.T) (*sql.DB, sqlmock.Sqlmock) {
	t.Helper()

	db, mock, err := sqlmock.New(sqlmock.QueryMatcherOption(sqlmock.QueryMatcherRegexp))
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	return db, mock
}

func expectTableExists(mock sqlmock.Sqlmock, exists bool) {
	mock.ExpectQuery(regexp.QuoteMeta(`
		SELECT EXISTS (
			SELECT FROM information_schema.tables 
			WHERE table_name = 'urls'
		)
	`)).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(exists))
}

func TestNewPostgresURLRepository_TableExists(t *testing.T) {
	db, mock := newMockDB(t)
	expectTableExists(mock, true)

	repo, err := NewPostgresURLRepository(db)
	require.NoError(t, err)
	require.NotNil(t, repo)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestNewPostgresURLRepository_TableMissing(t *testing.T) {
	db, mock := newMockDB(t)
	expectTableExists(mock, false)

	repo, err := NewPostgresURLRepository(db)
	require.Error(t, err)
	require.Nil(t, repo)
	require.Contains(t, err.Error(), "urls table does not exist")

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCreate_Success(t *testing.T) {
	db, mock := newMockDB(t)
	expectTableExists(mock, true)

	repo, err := NewPostgresURLRepository(db)
	require.NoError(t, err)

	u := &model.URL{ID: "id1", Original: "https://a.com", Short: "http://s/id1", UserID: "u1"}

	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO urls (id, original_url, short_url, user_id) VALUES ($1, $2, $3, $4)`)).
		WithArgs(u.ID, u.Original, u.Short, u.UserID).
		WillReturnResult(sqlmock.NewResult(1, 1))

	require.NoError(t, repo.Create(u))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCreate_UniqueViolation_ReturnsConflictWithExisting(t *testing.T) {
	db, mock := newMockDB(t)
	expectTableExists(mock, true)

	repo, err := NewPostgresURLRepository(db)
	require.NoError(t, err)

	u := &model.URL{ID: "id1", Original: "https://a.com", Short: "http://s/id1", UserID: "u1"}

	// Exec -> unique violation
	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO urls (id, original_url, short_url, user_id) VALUES ($1, $2, $3, $4)`)).
		WithArgs(u.ID, u.Original, u.Short, u.UserID).
		WillReturnError(&pgconn.PgError{Code: pgerrcode.UniqueViolation})

	// Затем repo.Create вызывает FindByOriginalURL
	rows := sqlmock.NewRows([]string{"id", "original_url", "short_url", "user_id", "is_deleted"}).
		AddRow("exist-id", u.Original, "http://s/exist-id", "exist-user", false)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, original_url, short_url, user_id, is_deleted FROM urls WHERE original_url = $1`)).
		WithArgs(u.Original).
		WillReturnRows(rows)

	err = repo.Create(u)
	require.Error(t, err)

	var ce *URLConflictError
	require.True(t, errors.As(err, &ce))
	require.NotNil(t, ce.ExistingURL)
	require.Equal(t, "exist-id", ce.ExistingURL.ID)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCreate_UniqueViolation_FindExistingFails(t *testing.T) {
	db, mock := newMockDB(t)
	expectTableExists(mock, true)

	repo, err := NewPostgresURLRepository(db)
	require.NoError(t, err)

	u := &model.URL{ID: "id1", Original: "https://a.com", Short: "http://s/id1", UserID: "u1"}

	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO urls (id, original_url, short_url, user_id) VALUES ($1, $2, $3, $4)`)).
		WithArgs(u.ID, u.Original, u.Short, u.UserID).
		WillReturnError(&pgconn.PgError{Code: pgerrcode.UniqueViolation})

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, original_url, short_url, user_id, is_deleted FROM urls WHERE original_url = $1`)).
		WithArgs(u.Original).
		WillReturnError(errors.New("db read fail"))

	err = repo.Create(u)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to find existing URL")

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCreate_NonUniqueError_Wrapped(t *testing.T) {
	db, mock := newMockDB(t)
	expectTableExists(mock, true)

	repo, err := NewPostgresURLRepository(db)
	require.NoError(t, err)

	u := &model.URL{ID: "id1", Original: "https://a.com", Short: "http://s/id1", UserID: "u1"}

	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO urls (id, original_url, short_url, user_id) VALUES ($1, $2, $3, $4)`)).
		WithArgs(u.ID, u.Original, u.Short, u.UserID).
		WillReturnError(errors.New("some db error"))

	err = repo.Create(u)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to insert URL")

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestFindByID_NoRows(t *testing.T) {
	db, mock := newMockDB(t)
	expectTableExists(mock, true)

	repo, err := NewPostgresURLRepository(db)
	require.NoError(t, err)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, original_url, short_url, user_id, is_deleted FROM urls WHERE id = $1`)).
		WithArgs("missing").
		WillReturnError(sql.ErrNoRows)

	got, err := repo.FindByID("missing")
	require.NoError(t, err)
	require.Nil(t, got)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestFindByID_Success(t *testing.T) {
	db, mock := newMockDB(t)
	expectTableExists(mock, true)

	repo, err := NewPostgresURLRepository(db)
	require.NoError(t, err)

	rows := sqlmock.NewRows([]string{"id", "original_url", "short_url", "user_id", "is_deleted"}).
		AddRow("id1", "https://a.com", "http://s/id1", "u1", false)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, original_url, short_url, user_id, is_deleted FROM urls WHERE id = $1`)).
		WithArgs("id1").
		WillReturnRows(rows)

	got, err := repo.FindByID("id1")
	require.NoError(t, err)
	require.NotNil(t, got)
	require.Equal(t, "id1", got.ID)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestFindByOriginalURL_NoRows(t *testing.T) {
	db, mock := newMockDB(t)
	expectTableExists(mock, true)

	repo, err := NewPostgresURLRepository(db)
	require.NoError(t, err)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, original_url, short_url, user_id, is_deleted FROM urls WHERE original_url = $1`)).
		WithArgs("https://missing.com").
		WillReturnError(sql.ErrNoRows)

	got, err := repo.FindByOriginalURL("https://missing.com")
	require.NoError(t, err)
	require.Nil(t, got)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestFindByUserID_Success(t *testing.T) {
	db, mock := newMockDB(t)
	expectTableExists(mock, true)

	repo, err := NewPostgresURLRepository(db)
	require.NoError(t, err)

	rows := sqlmock.NewRows([]string{"id", "original_url", "short_url", "user_id", "is_deleted"}).
		AddRow("1", "https://a.com", "http://s/1", "u1", false).
		AddRow("2", "https://b.com", "http://s/2", "u1", true)

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, original_url, short_url, user_id, is_deleted FROM urls WHERE user_id = $1`)).
		WithArgs("u1").
		WillReturnRows(rows)

	got, err := repo.FindByUserID("u1")
	require.NoError(t, err)
	require.Len(t, got, 2)
	require.Equal(t, "1", got[0].ID)
	require.Equal(t, true, got[1].IsDeleted)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateBatch_Success_MixedExistingAndNew(t *testing.T) {
	db, mock := newMockDB(t)
	expectTableExists(mock, true)

	repo, err := NewPostgresURLRepository(db)
	require.NoError(t, err)

	mock.ExpectBegin()

	// Prepare insert
	mock.ExpectPrepare(regexp.QuoteMeta(`INSERT INTO urls (id, original_url, short_url) VALUES ($1, $2, $3)`))

	// Prepare check
	mock.ExpectPrepare(regexp.QuoteMeta(`SELECT id, short_url FROM urls WHERE original_url = $1`))

	urls := []*model.URL{
		{ID: "n1", Original: "https://new.com", Short: "http://s/n1"},
		{ID: "n2", Original: "https://exists.com", Short: "http://s/n2"},
	}

	// 1) new.com -> no rows -> insert
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, short_url FROM urls WHERE original_url = $1`)).
		WithArgs("https://new.com").
		WillReturnError(sql.ErrNoRows)

	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO urls (id, original_url, short_url) VALUES ($1, $2, $3)`)).
		WithArgs("n1", "https://new.com", "http://s/n1").
		WillReturnResult(sqlmock.NewResult(1, 1))

	// 2) exists.com -> found -> update in-memory url.ID/Short
	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, short_url FROM urls WHERE original_url = $1`)).
		WithArgs("https://exists.com").
		WillReturnRows(sqlmock.NewRows([]string{"id", "short_url"}).AddRow("exist-id", "http://s/exist"))

	mock.ExpectCommit()

	require.NoError(t, repo.CreateBatch(urls))
	require.Equal(t, "n1", urls[0].ID)
	require.Equal(t, "exist-id", urls[1].ID)
	require.Equal(t, "http://s/exist", urls[1].Short)

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateBatch_CheckError(t *testing.T) {
	db, mock := newMockDB(t)
	expectTableExists(mock, true)

	repo, err := NewPostgresURLRepository(db)
	require.NoError(t, err)

	mock.ExpectBegin()
	mock.ExpectPrepare(regexp.QuoteMeta(`INSERT INTO urls (id, original_url, short_url) VALUES ($1, $2, $3)`))
	mock.ExpectPrepare(regexp.QuoteMeta(`SELECT id, short_url FROM urls WHERE original_url = $1`))

	urls := []*model.URL{
		{ID: "n1", Original: "https://x.com", Short: "http://s/n1"},
	}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, short_url FROM urls WHERE original_url = $1`)).
		WithArgs("https://x.com").
		WillReturnError(errors.New("db check fail"))

	err = repo.CreateBatch(urls)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to check existing URL")

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestCreateBatch_InsertError(t *testing.T) {
	db, mock := newMockDB(t)
	expectTableExists(mock, true)

	repo, err := NewPostgresURLRepository(db)
	require.NoError(t, err)

	mock.ExpectBegin()
	mock.ExpectPrepare(regexp.QuoteMeta(`INSERT INTO urls (id, original_url, short_url) VALUES ($1, $2, $3)`))
	mock.ExpectPrepare(regexp.QuoteMeta(`SELECT id, short_url FROM urls WHERE original_url = $1`))

	urls := []*model.URL{
		{ID: "n1", Original: "https://x.com", Short: "http://s/n1"},
	}

	mock.ExpectQuery(regexp.QuoteMeta(`SELECT id, short_url FROM urls WHERE original_url = $1`)).
		WithArgs("https://x.com").
		WillReturnError(sql.ErrNoRows)

	mock.ExpectExec(regexp.QuoteMeta(`INSERT INTO urls (id, original_url, short_url) VALUES ($1, $2, $3)`)).
		WithArgs("n1", "https://x.com", "http://s/n1").
		WillReturnError(errors.New("insert fail"))

	err = repo.CreateBatch(urls)
	require.Error(t, err)
	require.Contains(t, err.Error(), "failed to insert URL")

	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMarkAsDeleted_EmptyIDs_NoDBCall(t *testing.T) {
	db, mock := newMockDB(t)
	expectTableExists(mock, true)

	repo, err := NewPostgresURLRepository(db)
	require.NoError(t, err)

	require.NoError(t, repo.MarkAsDeleted(context.Background(), "u1", nil))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMarkAsDeleted_Success(t *testing.T) {
	db, mock := newMockDB(t)
	expectTableExists(mock, true)

	repo, err := NewPostgresURLRepository(db)
	require.NoError(t, err)

	mock.ExpectExec(regexp.QuoteMeta(`UPDATE urls SET is_deleted = TRUE WHERE user_id = $1 AND id = ANY($2)`)).
		WithArgs("u1", sqlmock.AnyArg()).
		WillReturnResult(sqlmock.NewResult(1, 2))

	require.NoError(t, repo.MarkAsDeleted(context.Background(), "u1", []string{"1", "2"}))
	require.NoError(t, mock.ExpectationsWereMet())
}
