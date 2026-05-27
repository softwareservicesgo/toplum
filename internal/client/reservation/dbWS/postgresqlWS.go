package dbWS

import (
	"context"
	"fmt"
	"restaurants/internal/appresult"
	"restaurants/pkg/client/postgresql"
	"restaurants/pkg/logging"
)

type RepositoryWS struct {
	client postgresql.Client
	logger *logging.Logger
}

func NewRepository(client postgresql.Client, logger *logging.Logger) *RepositoryWS {
	return &RepositoryWS{
		client: client,
		logger: logger,
	}
}

func (r *RepositoryWS) CheckRestaurant(ctx context.Context, restaurantId int, clientId int) error {
	var exists bool

	queryRestaurant := `SELECT EXISTS(SELECT 1 FROM restaurants WHERE id = $1)`
	err := r.client.QueryRow(ctx, queryRestaurant, restaurantId).Scan(&exists)
	if err != nil {
		fmt.Println("error :", err)
		return appresult.ErrInternalServer
	}
	if !exists {
		return appresult.ErrNotFoundType(restaurantId, "restaurant")
	}

	queryClient := `SELECT EXISTS(SELECT 1 FROM users WHERE id = $1 AND (restaurant_id = $2 OR role = 'admin'))`
	err = r.client.QueryRow(ctx, queryClient, clientId, restaurantId).Scan(&exists)
	if err != nil {
		fmt.Println("error :", err)
		return appresult.ErrInternalServer
	}
	if !exists {
		return appresult.ErrNotFoundType(clientId, "users")
	}

	return nil
}

// func (r *RepositoryWS) AvailabilityDay(ctx context.Context, restaurantId int, personCount reservation.AvailabilityDayReq, today, endDate time.Time) (*[]reservation.AvailabilityDay, error) {
// 	var (
// 		availabilityDays []reservation.AvailabilityDay
// 		maxTableCount    int
// 	)
// 	query := `
// 	    	SELECT seats, table_count
// 	    FROM restaurant_tables
// 	    WHERE restaurant_id = $1
// 		AND seats >= $2
// 	`
// 	rows, err := r.client.Query(ctx, query, restaurantId, personCount.PersonCount)
// 	if err != nil {
// 		fmt.Println("error :", err)
// 		return nil, err
// 	}
// 	defer rows.Close()

// 	for rows.Next() {
// 		var (
// 			seats      int
// 			tableCount int
// 		)
// 		if err := rows.Scan(&seats, &tableCount); err != nil {
// 			fmt.Println("error :", err)
// 			return nil, err
// 		}
// 		maxTableCount += seats * tableCount
// 	}

// 	for date := today; !date.After(endDate); date = date.AddDate(0, 0, 1) {
// 		availabilityTime := reservation.AvailabilityTimeReq{
// 			PersonCount: personCount.PersonCount,
// 			Day:         date,
// 		}

// 		_, available, err := r.AvailabilityTimes(ctx, restaurantId, availabilityTime)
// 		if err != nil {
// 			fmt.Println("error: ", err)
// 			return nil, err
// 		}

// 		availabilityDay := reservation.AvailabilityDay{
// 			Day:       date.Format("2006-01-02"),
// 			Available: *available,
// 		}
// 		availabilityDays = append(availabilityDays, availabilityDay)
// 	}
// 	return &availabilityDays, nil
// }

// func (r *RepositoryWS) AvailabilityTimes(ctx context.Context, restaurantId int, availabilityTime reservation.AvailabilityTimeReq) (*[]reservation.AvailabilityTime, *bool, error) {
// 	var (
// 		availabilityTimes []reservation.AvailabilityTime
// 		openTime          time.Time
// 		closesTime        time.Time
// 		isDayEmpty        bool
// 	)
// 	query := `
// 	    	SELECT opens_time, closes_time
// 	    FROM restaurants
// 	    WHERE id = $1
// 	`
// 	err := r.client.QueryRow(ctx, query, restaurantId).Scan(&openTime, &closesTime)
// 	if err != nil {
// 		fmt.Println("error :", err)
// 		return nil, nil, err
// 	}

// 	query = `
// 	    	SELECT id, seats, table_count
// 	    FROM restaurant_tables
// 	    WHERE restaurant_id = $1
// 	`
// 	rows, err := r.client.Query(ctx, query, restaurantId)
// 	if err != nil {
// 		fmt.Println("error :", err)
// 		return nil, nil, err
// 	}
// 	defer rows.Close()

// 	for checkTime := openTime; !checkTime.After(closesTime); checkTime = checkTime.Add(time.Minute * 30) {
// 		//isEmpty := false
// 		//isEmpty, err := AvailabilityTime(restaurantId, availabilityTime.Day, checkTime, tables, availabilityTime.PersonCount, r, ctx)

// 		if err != nil {
// 			return nil, nil, err
// 		}
// 		// if *isEmpty {
// 		// 	isDayEmpty = true
// 		// }
// 		// for _, table := range tables {
// 		// 	if table.Seat >= availabilityTime.PersonCount {
// 		// 		var (
// 		// 			countUseTable int
// 		// 		)
// 		// 		query := `
// 		// 					SELECT COUNT(*)
// 		// 				FROM reservations
// 		// 				WHERE restaurant_id = $1
// 		// 				AND reservation_date::date = $2
// 		// 				AND status IN ('pending','confirmed')
// 		// 				AND restaurant_table_id = $3
// 		// 				AND reservation_date::time <= $4
// 		//  		 `

// 		// 		err := r.client.QueryRow(ctx, query, restaurantId, availabilityTime.Day, table.Id, checkTime).
// 		// 			Scan(&countUseTable)
// 		// 		if err != nil {
// 		// 			fmt.Println("error :", err)
// 		// 			return nil, nil, err
// 		// 		}

// 		// 		if table.TableCount-countUseTable > 0 {
// 		// 			isDayEmpty = true
// 		// 			isEmpty = true
// 		// 		}
// 		// 	}
// 		// }
// 		availabilityT := reservation.AvailabilityTime{
// 			Time:      checkTime.Format("15:04"),
// 			Available: *isEmpty,
// 		}
// 		availabilityTimes = append(availabilityTimes, availabilityT)
// 	}
// 	return &availabilityTimes, &isDayEmpty, nil
// }

// func AvailabilityTime(restaurantId int, day time.Time, checkTime time.Time, personCount int, r *RepositoryWS, ctx context.Context) (*bool, error) {
// 	isEmpty := false
// 	for _, table := range tables {
// 		if table.Seat >= personCount {
// 			var (
// 				countUseTable int
// 			)
// 			query := `
// 							SELECT COUNT(*)
// 						FROM reservations
// 						WHERE restaurant_id = $1
// 						AND reservation_date::date = $2
// 						AND status IN ('pending','confirmed')
// 						AND restaurant_table_id = $3
// 						AND reservation_date::time <= $4
// 		 		 `

// 			err := r.client.QueryRow(ctx, query, restaurantId, day, table.Id, checkTime).
// 				Scan(&countUseTable)
// 			if err != nil {
// 				fmt.Println("error :", err)
// 				return nil, err
// 			}

// 			fmt.Println("++++", restaurantId, " -- ", countUseTable)

// 			if table.TableCount-countUseTable > 0 {
// 				isEmpty = true
// 			}
// 		}
// 	}
// 	return &isEmpty, nil
// }
