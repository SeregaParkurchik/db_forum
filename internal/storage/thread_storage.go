package storage

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"

	"hardhw/internal/models"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ThreadStorage interface {
	GetThreadIDBySlugOrID(ctx context.Context, slugOrID string) (int64, error)
	CheckParentPostExistsInThread(ctx context.Context, parentID int64, threadID int64) (bool, error)
	CreatePosts(ctx context.Context, posts []*models.Post) ([]models.Post, error)
	UpdateThreadVote(ctx context.Context, threadID int64, nickname string, voice int) (*models.Thread, error)
	GetThreadBySlugOrID(ctx context.Context, slugOrID string) (*models.Thread, error)
	GetFlatThreadPosts(ctx context.Context, threadID int64, limit int, since int64, desc bool) ([]models.Post, error)
	GetTreeThreadPosts(ctx context.Context, threadID int64, limit int, since int64, desc bool) ([]models.Post, error)
	GetParentTreeThreadPosts(ctx context.Context, threadId int64, limit int, since int64, desc bool) ([]models.Post, error)
	UpdateThread(ctx context.Context, slugOrID string, updateData models.ThreadUpdate) (models.Thread, error)
	GetThreadByID(ctx context.Context, id int64) (*models.Thread, error)
}

type postgresThreadStorage struct {
	pool *pgxpool.Pool
}

func NewPostgresThreadStorage(pool *pgxpool.Pool) ThreadStorage {
	return &postgresThreadStorage{pool: pool}
}

func (s *postgresThreadStorage) GetThreadIDBySlugOrID(ctx context.Context, slugOrID string) (int64, error) {
	var threadID int64
	query := `SELECT id FROM threads WHERE slug = $1 OR id = $2::int`

	idInt, err := strconv.ParseInt(slugOrID, 10, 64)
	if err != nil {

		idInt = 0
	}

	err = s.pool.QueryRow(ctx, query, slugOrID, idInt).Scan(&threadID)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, models.ErrNotFound
		}
		return 0, fmt.Errorf("failed to get thread ID by slug or ID: %w", err)
	}
	return threadID, nil
}

func (s *postgresThreadStorage) CheckParentPostExistsInThread(ctx context.Context, parentID int64, threadID int64) (bool, error) {
	query := `SELECT EXISTS(SELECT 1 FROM posts WHERE id = $1 AND thread_id = $2)`
	var exists bool
	err := s.pool.QueryRow(ctx, query, parentID, threadID).Scan(&exists)
	if err != nil {
		return false, fmt.Errorf("failed to check parent post existence: %w", err)
	}
	return exists, nil
}

func (s *postgresThreadStorage) CreatePosts(ctx context.Context, posts []*models.Post) ([]models.Post, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	if len(posts) == 0 {
		return []models.Post{}, nil
	}

	var threadForumSlug string
	getThreadForumQuery := `SELECT forum FROM threads WHERE id = $1`
	err = tx.QueryRow(ctx, getThreadForumQuery, posts[0].Thread).Scan(&threadForumSlug)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get forum slug for thread %d: %w", posts[0].Thread, err)
	}

	insertQueryPrefix := `INSERT INTO posts(author, created, forum, message, parent, thread_id) VALUES ` // Используем thread_id
	valuesBuilder := strings.Builder{}
	var queryArgs []interface{}
	created := time.Now()

	for i, p := range posts {
		if i > 0 {
			valuesBuilder.WriteString(",")
		}
		valuesBuilder.WriteString(fmt.Sprintf("($%d, $%d, $%d, $%d, $%d, $%d)",
			len(queryArgs)+1, len(queryArgs)+2, len(queryArgs)+3, len(queryArgs)+4, len(queryArgs)+5, len(queryArgs)+6))

		queryArgs = append(queryArgs, p.Author, created, threadForumSlug, p.Message, p.Parent, p.Thread)

		p.Created = created
		p.Forum = threadForumSlug
	}

	fullInsertQuery := insertQueryPrefix + valuesBuilder.String() + ` RETURNING id, parent, author, message, is_edited, forum, thread_id, created`

	rows, err := tx.Query(ctx, fullInsertQuery, queryArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to execute batch insert for posts: %w", err)
	}
	defer rows.Close()

	newlyCreatedPostsMap := make(map[int64]*models.Post)
	var postsInOriginalOrder []*models.Post

	for rows.Next() {
		var createdPost models.Post
		err := rows.Scan(
			&createdPost.ID, &createdPost.Parent, &createdPost.Author, &createdPost.Message,
			&createdPost.IsEdited, &createdPost.Forum, &createdPost.Thread, &createdPost.Created,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan created post after batch insert: %w", err)
		}
		createdPost.Path = []int64{}
		createdPost.RootParentID = 0
		newlyCreatedPostsMap[createdPost.ID] = &createdPost
		postsInOriginalOrder = append(postsInOriginalOrder, &createdPost)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error after batch insert: %w", err)
	}

	postsForProcessing := make([]*models.Post, 0, len(newlyCreatedPostsMap))
	for _, p := range newlyCreatedPostsMap {
		postsForProcessing = append(postsForProcessing, p)
	}

	sort.Slice(postsForProcessing, func(i, j int) bool {
		if postsForProcessing[i].Parent == 0 && postsForProcessing[j].Parent != 0 {
			return true
		}
		if postsForProcessing[i].Parent != 0 && postsForProcessing[j].Parent == 0 {
			return false
		}
		if postsForProcessing[i].Parent != postsForProcessing[j].Parent {
			return postsForProcessing[i].Parent < postsForProcessing[j].Parent
		}
		return postsForProcessing[i].ID < postsForProcessing[j].ID
	})

	updateBatch := &pgx.Batch{}

	for _, currentPost := range postsForProcessing {
		var finalPath []int64
		var finalRootParentID int64

		if currentPost.Parent == 0 {
			finalPath = []int64{currentPost.ID}
			finalRootParentID = currentPost.ID
		} else {
			parentPost, foundInBatch := newlyCreatedPostsMap[currentPost.Parent]
			if foundInBatch {
				if parentPost.Path == nil {
					return nil, fmt.Errorf("internal error: parent post %d path not set before child %d", parentPost.ID, currentPost.ID)
				}
				finalPath = append(parentPost.Path, currentPost.ID)
				finalRootParentID = parentPost.RootParentID
			} else {
				var parentPath []int64
				var parentRootID int64
				err := tx.QueryRow(ctx, "SELECT path, root_parent_id FROM posts WHERE id = $1 AND thread_id = $2", currentPost.Parent, currentPost.Thread).Scan(&parentPath, &parentRootID)
				if err != nil {
					if errors.Is(err, pgx.ErrNoRows) {
						return nil, models.ErrParentNotFound
					}
					return nil, fmt.Errorf("failed to get parent path for post %d (parent %d) during update: %w", currentPost.ID, currentPost.Parent, err)
				}
				finalPath = append(parentPath, currentPost.ID)
				finalRootParentID = parentRootID
			}
		}
		currentPost.Path = finalPath
		currentPost.RootParentID = finalRootParentID
		updateBatch.Queue("UPDATE posts SET path = $1, root_parent_id = $2 WHERE id = $3", finalPath, finalRootParentID, currentPost.ID)
	}

	updateBr := tx.SendBatch(ctx, updateBatch)
	err = updateBr.Close()
	if err != nil {
		return nil, fmt.Errorf("failed to execute update batch for posts path/root_parent_id: %w", err)
	}

	updateForumPostsCountQuery := `
        UPDATE forums
        SET posts = posts + $1
        WHERE slug = $2`
	_, err = tx.Exec(ctx, updateForumPostsCountQuery, len(posts), threadForumSlug)
	if err != nil {
		return nil, fmt.Errorf("failed to update forum posts count: %w", err)
	}

	forumUsersBatch := &pgx.Batch{}
	uniqueAuthors := make(map[string]struct{})
	for _, p := range postsInOriginalOrder {
		if _, seen := uniqueAuthors[p.Author]; !seen {
			forumUsersBatch.Queue(
				`INSERT INTO forum_users (forum_slug, user_nickname) VALUES ($1, $2) ON CONFLICT (forum_slug, user_nickname) DO NOTHING`,
				threadForumSlug, p.Author,
			)
			uniqueAuthors[p.Author] = struct{}{}
		}
	}

	if forumUsersBatch.Len() > 0 {
		forumUsersBr := tx.SendBatch(ctx, forumUsersBatch)
		err = forumUsersBr.Close()
		if err != nil {
			return nil, fmt.Errorf("failed to update forum_users batch: %w", err)
		}
	}

	err = tx.Commit(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to commit transaction: %w", err)
	}

	result := make([]models.Post, len(postsInOriginalOrder))
	for i, p := range postsInOriginalOrder {
		result[i] = *p
	}
	return result, nil
}

func (s *postgresThreadStorage) UpdateThreadVote(ctx context.Context, threadID int64, nickname string, voice int) (*models.Thread, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to begin transaction for voting: %w", err)
	}
	defer tx.Rollback(ctx)

	var oldVoice int
	err = tx.QueryRow(ctx, `SELECT voice FROM votes WHERE thread_id = $1 AND user_nickname = $2`, threadID, nickname).Scan(&oldVoice)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			_, err = tx.Exec(ctx, `INSERT INTO votes (thread_id, user_nickname, voice) VALUES ($1, $2, $3)`, threadID, nickname, voice)
			if err != nil {
				return nil, fmt.Errorf("failed to insert new vote: %w", err)
			}
			_, err = tx.Exec(ctx, `UPDATE threads SET votes = votes + $1 WHERE id = $2`, voice, threadID)
			if err != nil {
				return nil, fmt.Errorf("failed to update thread votes for new vote: %w", err)
			}
		} else {
			return nil, fmt.Errorf("failed to check existing vote: %w", err)
		}
	} else {
		if oldVoice != voice {
			_, err = tx.Exec(ctx, `UPDATE votes SET voice = $1 WHERE thread_id = $2 AND user_nickname = $3`, voice, threadID, nickname)
			if err != nil {
				return nil, fmt.Errorf("failed to update existing vote: %w", err)
			}
			voteDelta := voice - oldVoice
			_, err = tx.Exec(ctx, `UPDATE threads SET votes = votes + $1 WHERE id = $2`, voteDelta, threadID)
			if err != nil {
				return nil, fmt.Errorf("failed to update thread votes for changed vote: %w", err)
			}
		}
	}

	var updatedThread models.Thread
	err = tx.QueryRow(ctx, `
        SELECT id, title, author, forum, message, votes, slug, created
        FROM threads
        WHERE id = $1`, threadID).Scan(
		&updatedThread.ID,
		&updatedThread.Title,
		&updatedThread.Author,
		&updatedThread.Forum,
		&updatedThread.Message,
		&updatedThread.Votes,
		&updatedThread.Slug,
		&updatedThread.Created,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("failed to retrieve updated thread after vote: %w", err)
	}

	err = tx.Commit(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to commit vote transaction: %w", err)
	}

	return &updatedThread, nil
}

func (s *postgresThreadStorage) GetThreadBySlugOrID(ctx context.Context, slugOrID string) (*models.Thread, error) {
	var thread models.Thread
	query := `
        SELECT id, title, author, forum, message, votes, slug, created
        FROM threads
        WHERE slug = $1 OR id = $2::int`

	idInt, err := strconv.ParseInt(slugOrID, 10, 64)
	if err != nil {
		idInt = 0
	}

	err = s.pool.QueryRow(ctx, query, slugOrID, idInt).Scan(
		&thread.ID,
		&thread.Title,
		&thread.Author,
		&thread.Forum,
		&thread.Message,
		&thread.Votes,
		&thread.Slug,
		&thread.Created,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get thread by slug or ID: %w", err)
	}
	return &thread, nil
}

func (s *postgresThreadStorage) GetFlatThreadPosts(ctx context.Context, threadID int64, limit int, since int64, desc bool) ([]models.Post, error) {

	baseQuery := `
        SELECT id, parent, author, message, is_edited, forum, thread_id, created, path, root_parent_id
        FROM posts
        WHERE thread_id = $1
    `
	args := []interface{}{threadID}
	argPos := 2

	if since > 0 {
		var sinceCreated time.Time
		var sinceID int64
		err := s.pool.QueryRow(ctx, "SELECT created, id FROM posts WHERE id = $1 AND thread_id = $2", since, threadID).Scan(&sinceCreated, &sinceID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return []models.Post{}, nil
			}
			return nil, fmt.Errorf("failed to get 'created' and 'id' for since post %d: %w", since, err)
		}

		if desc {
			baseQuery += fmt.Sprintf(" AND (created < $%d OR (created = $%d AND id < $%d))", argPos, argPos+1, argPos+2)
			args = append(args, sinceCreated, sinceCreated, sinceID)
			argPos += 3
		} else {
			baseQuery += fmt.Sprintf(" AND (created > $%d OR (created = $%d AND id > $%d))", argPos, argPos+1, argPos+2)
			args = append(args, sinceCreated, sinceCreated, sinceID)
			argPos += 3
		}
	}

	orderBy := " ORDER BY created ASC, id ASC"
	if desc {
		orderBy = " ORDER BY created DESC, id DESC"
	}

	limitClause := fmt.Sprintf(" LIMIT NULLIF($%d, 0)", argPos)
	args = append(args, limit)

	query := baseQuery + orderBy + limitClause

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query flat posts: %w", err)
	}
	defer rows.Close()

	posts := make([]models.Post, 0, limit)
	for rows.Next() {
		var post models.Post
		if err := rows.Scan(
			&post.ID, &post.Parent, &post.Author, &post.Message, &post.IsEdited,
			&post.Forum, &post.Thread, &post.Created, &post.Path, &post.RootParentID,
		); err != nil {
			return nil, fmt.Errorf("failed to scan post in flat mode: %w", err)
		}
		posts = append(posts, post)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error in flat mode: %w", err)
	}

	return posts, nil
}

func (s *postgresThreadStorage) GetTreeThreadPosts(ctx context.Context, threadID int64, limit int, since int64, desc bool) ([]models.Post, error) {
	baseQuery := `
        SELECT id, parent, author, message, is_edited, forum, thread_id, created, path, root_parent_id
        FROM posts
        WHERE thread_id = $1
    `
	args := []interface{}{threadID}
	argPos := 2

	if since > 0 {
		var sincePath []int64
		var sinceID int64
		err := s.pool.QueryRow(ctx, "SELECT path, id FROM posts WHERE id = $1 AND thread_id = $2", since, threadID).Scan(&sincePath, &sinceID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return []models.Post{}, nil
			}
			return nil, fmt.Errorf("failed to get 'path' for since post %d: %w", since, err)
		}

		if desc {

			baseQuery += fmt.Sprintf(" AND (path < $%d OR (path = $%d AND id < $%d))", argPos, argPos+1, argPos+2)
			args = append(args, sincePath, sincePath, sinceID)
			argPos += 3
		} else {

			baseQuery += fmt.Sprintf(" AND (path > $%d OR (path = $%d AND id > $%d))", argPos, argPos+1, argPos+2)
			args = append(args, sincePath, sincePath, sinceID)
			argPos += 3
		}
	}

	orderBy := " ORDER BY path ASC, id ASC"
	if desc {
		orderBy = " ORDER BY path DESC, id DESC"
	}

	limitClause := fmt.Sprintf(" LIMIT NULLIF($%d, 0)", argPos)
	args = append(args, limit)

	query := baseQuery + orderBy + limitClause

	rows, err := s.pool.Query(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("failed to query tree posts: %w", err)
	}
	defer rows.Close()

	posts := make([]models.Post, 0, limit)
	for rows.Next() {
		var post models.Post
		if err := rows.Scan(
			&post.ID, &post.Parent, &post.Author, &post.Message, &post.IsEdited,
			&post.Forum, &post.Thread, &post.Created, &post.Path, &post.RootParentID,
		); err != nil {
			return nil, fmt.Errorf("failed to scan post in tree mode: %w", err)
		}
		posts = append(posts, post)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error in tree mode: %w", err)
	}

	return posts, nil
}

func (s *postgresThreadStorage) UpdateThread(ctx context.Context, slugOrID string, updateData models.ThreadUpdate) (models.Thread, error) {

	threadID, err := s.GetThreadIDBySlugOrID(ctx, slugOrID)
	if err != nil {
		return models.Thread{}, err
	}

	var thread models.Thread
	var args []interface{}
	paramCounter := 1
	setClauses := ""

	if updateData.Title != nil {
		setClauses += fmt.Sprintf(`title = $%d, `, paramCounter)
		args = append(args, *updateData.Title)
		paramCounter++
	}
	if updateData.Message != nil {
		setClauses += fmt.Sprintf(`message = $%d, `, paramCounter)
		args = append(args, *updateData.Message)
		paramCounter++
	}

	if setClauses == "" {

		query := `SELECT id, title, author, forum, message, votes, slug, created FROM threads WHERE id = $1`
		row := s.pool.QueryRow(ctx, query, threadID)
		err := row.Scan(
			&thread.ID,
			&thread.Title,
			&thread.Author,
			&thread.Forum,
			&thread.Message,
			&thread.Votes,
			&thread.Slug,
			&thread.Created,
		)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return models.Thread{}, models.ErrNotFound
			}
			return models.Thread{}, fmt.Errorf("failed to get thread after no updates: %w", err)
		}
		return thread, nil
	}

	setClauses = setClauses[:len(setClauses)-2]

	args = append(args, threadID)
	whereClauseParam := paramCounter

	query := fmt.Sprintf(`
        UPDATE threads
        SET %s
        WHERE id = $%d
        RETURNING id, title, author, forum, message, votes, slug, created`,
		setClauses, whereClauseParam)

	row := s.pool.QueryRow(ctx, query, args...)

	err = row.Scan(
		&thread.ID,
		&thread.Title,
		&thread.Author,
		&thread.Forum,
		&thread.Message,
		&thread.Votes,
		&thread.Slug,
		&thread.Created,
	)

	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {

			return models.Thread{}, models.ErrNotFound
		}
		return models.Thread{}, fmt.Errorf("failed to update thread: %w", err)
	}

	return thread, nil
}

func (s *postgresThreadStorage) GetThreadByID(ctx context.Context, id int64) (*models.Thread, error) {
	query := `
		SELECT id, title, author, forum, message, votes, slug, created
		FROM threads
		WHERE id = $1
	`
	thread := &models.Thread{}
	var slug sql.NullString
	err := s.pool.QueryRow(ctx, query, id).Scan(
		&thread.ID,
		&thread.Title,
		&thread.Author,
		&thread.Forum,
		&thread.Message,
		&thread.Votes,
		&slug,
		&thread.Created,
	)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, models.ErrNotFound
		}
		return nil, fmt.Errorf("failed to get thread by ID: %w", err)
	}
	if slug.Valid {
		thread.Slug = &slug.String
	}
	return thread, nil
}

func (s *postgresThreadStorage) GetParentTreeThreadPosts(ctx context.Context, threadID int64, limit int, since int64, desc bool) ([]models.Post, error) {

	rootPostsSelectClause := `SELECT id FROM posts WHERE thread_id = $1 AND parent = 0`
	rootArgs := []interface{}{threadID}
	rootArgPos := 2

	if since > 0 {
		var sinceRootParentID int64
		err := s.pool.QueryRow(ctx, "SELECT root_parent_id FROM posts WHERE id = $1 AND thread_id = $2", since, threadID).Scan(&sinceRootParentID)
		if err != nil {
			if errors.Is(err, pgx.ErrNoRows) {
				return []models.Post{}, nil
			}
			return nil, fmt.Errorf("failed to get root_parent_id for since post %d: %w", since, err)
		}

		if desc {
			rootPostsSelectClause += fmt.Sprintf(" AND id < $%d", rootArgPos)
		} else {
			rootPostsSelectClause += fmt.Sprintf(" AND id > $%d", rootArgPos)
		}
		rootArgs = append(rootArgs, sinceRootParentID)
		rootArgPos++
	}

	rootOrderBy := " ORDER BY id ASC"
	if desc {
		rootOrderBy = " ORDER BY id DESC"
	}

	rootLimitClause := fmt.Sprintf(" LIMIT NULLIF($%d, 0)", rootArgPos)
	rootArgs = append(rootArgs, limit)

	rootIDsQuery := rootPostsSelectClause + rootOrderBy + rootLimitClause

	rootRows, err := s.pool.Query(ctx, rootIDsQuery, rootArgs...)
	if err != nil {
		return nil, fmt.Errorf("failed to query root posts for parent_tree pagination: %w", err)
	}
	defer rootRows.Close()

	var rootPostIDs []int64
	for rootRows.Next() {
		var id int64
		if err := rootRows.Scan(&id); err != nil {
			return nil, fmt.Errorf("failed to scan root post ID for parent_tree: %w", err)
		}
		rootPostIDs = append(rootPostIDs, id)
	}
	if err := rootRows.Err(); err != nil {
		return nil, fmt.Errorf("rows error in root post IDs query: %w", err)
	}

	if len(rootPostIDs) == 0 {
		return []models.Post{}, nil
	}

	mainQuery := `
        SELECT id, parent, author, message, is_edited, forum, thread_id, created, path, root_parent_id
        FROM posts
        WHERE thread_id = $1 AND root_parent_id = ANY($2)
    `

	mainArgs := []interface{}{threadID, rootPostIDs}

	mainOrderBy := " ORDER BY root_parent_id ASC, path ASC, id ASC"
	if desc {
		mainOrderBy = " ORDER BY root_parent_id DESC, path ASC, id DESC"
	}

	mainQuery += mainOrderBy

	rows, err := s.pool.Query(ctx, mainQuery, mainArgs...)
	if err != nil {
		return nil, fmt.Errorf("database query failed for GetParentTreeThreadPosts: %w", err)
	}
	defer rows.Close()

	posts := make([]models.Post, 0)
	for rows.Next() {
		var post models.Post
		if err = rows.Scan(
			&post.ID, &post.Parent, &post.Author, &post.Message, &post.IsEdited,
			&post.Forum, &post.Thread, &post.Created, &post.Path, &post.RootParentID,
		); err != nil {
			return nil, fmt.Errorf("failed to scan row for GetParentTreeThreadPosts: %w", err)
		}
		posts = append(posts, post)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("error iterating rows for GetParentTreeThreadPosts: %w", err)
	}

	return posts, nil
}
