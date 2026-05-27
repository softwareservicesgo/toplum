package db

import (
	"context"
	"restaurants/internal/appresult"
	"restaurants/internal/client/offer"
	"restaurants/pkg/client/postgresql"
	"restaurants/pkg/logging"
)

type repository struct {
	client postgresql.Client
	logger *logging.Logger
}

func NewRepository(
	client postgresql.Client,
	logger *logging.Logger,
) offer.Repository {
	return &repository{
		client: client,
		logger: logger,
	}
}

func (r *repository) Create(
	ctx context.Context,
	req offer.CreateOfferReq,
	userId int,
) (*offer.OfferDTO, error) {

	var resp offer.OfferDTO

	q := `
		INSERT INTO offers (user_id, content)
		VALUES ($1, $2)
		RETURNING id, content
	`

	err := r.client.QueryRow(
		ctx,
		q,
		userId,
		req.Content,
	).Scan(
		&resp.Id,
		&resp.Content,
	)

	if err != nil {
		return nil, appresult.ErrInternalServer
	}

	q = `
		SELECT name
		FROM users
		WHERE id = $1
	`

	err = r.client.QueryRow(ctx, q, userId).Scan(&resp.UserName)
	if err != nil {
		return nil, appresult.ErrInternalServer
	}

	return &resp, nil
}

func (r *repository) GetAll(
	ctx context.Context,
	limit,
	page int,
) (*offer.GetAll, error) {
	var result offer.GetAll

	if page <= 0 {
		page = 1
	}

	offset := (page - 1) * limit

	q := `
		SELECT
			o.id,
			u.name,
			o.content
		FROM offers o
		JOIN users u ON u.id = o.user_id
		ORDER BY o.created_at DESC
		LIMIT $1 OFFSET $2
	`

	rows, err := r.client.Query(ctx, q, limit, offset)
	if err != nil {
		return nil, appresult.ErrInternalServer
	}
	defer rows.Close()

	for rows.Next() {
		var item offer.OfferDTO

		err = rows.Scan(
			&item.Id,
			&item.UserName,
			&item.Content,
		)
		if err != nil {
			return nil, appresult.ErrInternalServer
		}

		result.Offers = append(result.Offers, item)
	}

	qCount := `
		SELECT COUNT(*)
		FROM offers
	`

	err = r.client.QueryRow(ctx, qCount).Scan(&result.Count)
	if err != nil {
		return nil, appresult.ErrInternalServer
	}

	return &result, nil
}

func (r *repository) Clean(
	ctx context.Context,
) error {

	q := `
		DELETE FROM offers
	`

	_, err := r.client.Exec(ctx, q)
	if err != nil {
		return appresult.ErrInternalServer
	}

	return nil
}
