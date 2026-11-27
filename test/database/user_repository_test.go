package database_test

import (
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/smarttransit/sms-auth-backend/internal/database"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func setupUserRepoTest(t *testing.T) (*database.UserRepository, sqlmock.Sqlmock, func()) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)

	sqlxDB := sqlx.NewDb(db, "sqlmock")
	postgresDB := &database.PostgresDB{DB: sqlxDB}
	repo := database.NewUserRepository(postgresDB)

	return repo, mock, func() { db.Close() }
}

func TestCreateUser(t *testing.T) {
	repo, mock, cleanup := setupUserRepoTest(t)
	defer cleanup()

	t.Run("Success", func(t *testing.T) {
		phone := "+94712345678"

		mock.ExpectExec(`INSERT INTO users`).
			WithArgs(
				sqlmock.AnyArg(), phone, sqlmock.AnyArg(), "active",
				false, true, false, sqlmock.AnyArg(), sqlmock.AnyArg(),
			).
			WillReturnResult(sqlmock.NewResult(1, 1))

		user, err := repo.CreateUser(phone)
		require.NoError(t, err)
		assert.NotNil(t, user)
		assert.Equal(t, phone, user.Phone)
		assert.Equal(t, "active", user.Status)
		assert.True(t, user.PhoneVerified)
		assert.False(t, user.ProfileCompleted)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})

	t.Run("Database Error", func(t *testing.T) {
		phone := "+94712345678"

		mock.ExpectExec(`INSERT INTO users`).
			WithArgs(
				sqlmock.AnyArg(), phone, sqlmock.AnyArg(), "active",
				false, true, false, sqlmock.AnyArg(), sqlmock.AnyArg(),
			).
			WillReturnError(fmt.Errorf("database error"))

		user, err := repo.CreateUser(phone)
		assert.Error(t, err)
		assert.Nil(t, user)
		assert.Contains(t, err.Error(), "failed to create user")

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})

	t.Run("Duplicate Phone", func(t *testing.T) {
		phone := "+94712345678"

		mock.ExpectExec(`INSERT INTO users`).
			WithArgs(
				sqlmock.AnyArg(), phone, sqlmock.AnyArg(), "active",
				false, true, false, sqlmock.AnyArg(), sqlmock.AnyArg(),
			).
			WillReturnError(fmt.Errorf("duplicate key value violates unique constraint"))

		user, err := repo.CreateUser(phone)
		assert.Error(t, err)
		assert.Nil(t, user)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})
}

func TestGetUserByPhone(t *testing.T) {
	repo, mock, cleanup := setupUserRepoTest(t)
	defer cleanup()

	t.Run("Success", func(t *testing.T) {
		phone := "+94712345678"
		userID := uuid.New()
		now := time.Now()

		mock.ExpectQuery(`SELECT (.+) FROM users WHERE phone`).
			WithArgs(phone).
			WillReturnRows(sqlmock.NewRows([]string{
				"id", "phone", "first_name", "last_name", "email", "address",
				"city", "postal_code", "roles", "status", "profile_completed",
				"phone_verified", "email_verified", "created_at", "updated_at",
			}).AddRow(
				userID, phone, "John", "Doe", "john@example.com", "123 Main St",
				"Colombo", "10100", []byte(`{"passenger"}`), "active", true,
				true, true, now, now,
			))

		user, err := repo.GetUserByPhone(phone)
		require.NoError(t, err)
		assert.NotNil(t, user)
		assert.Equal(t, userID, user.ID)
		assert.Equal(t, phone, user.Phone)
		assert.Equal(t, "John", user.FirstName.String)
		assert.Equal(t, "Doe", user.LastName.String)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})

	t.Run("User Not Found", func(t *testing.T) {
		phone := "+94712345678"

		mock.ExpectQuery(`SELECT (.+) FROM users WHERE phone`).
			WithArgs(phone).
			WillReturnError(sql.ErrNoRows)

		user, err := repo.GetUserByPhone(phone)
		require.NoError(t, err)
		assert.Nil(t, user)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})

	t.Run("Database Error", func(t *testing.T) {
		phone := "+94712345678"

		mock.ExpectQuery(`SELECT (.+) FROM users WHERE phone`).
			WithArgs(phone).
			WillReturnError(fmt.Errorf("database error"))

		user, err := repo.GetUserByPhone(phone)
		assert.Error(t, err)
		assert.Nil(t, user)
		assert.Contains(t, err.Error(), "failed to get user by phone")

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})
}

func TestGetUserByID(t *testing.T) {
	repo, mock, cleanup := setupUserRepoTest(t)
	defer cleanup()

	t.Run("Success", func(t *testing.T) {
		userID := uuid.New()
		phone := "+94712345678"
		now := time.Now()

		mock.ExpectQuery(`SELECT (.+) FROM users WHERE id`).
			WithArgs(userID).
			WillReturnRows(sqlmock.NewRows([]string{
				"id", "phone", "first_name", "last_name", "email", "address",
				"city", "postal_code", "roles", "status", "profile_completed",
				"phone_verified", "email_verified", "created_at", "updated_at",
			}).AddRow(
				userID, phone, "Jane", "Smith", "jane@example.com", "456 Oak Ave",
				"Kandy", "20000", []byte(`{"passenger","driver"}`), "active", true,
				true, false, now, now,
			))

		user, err := repo.GetUserByID(userID)
		require.NoError(t, err)
		assert.NotNil(t, user)
		assert.Equal(t, userID, user.ID)
		assert.Equal(t, "Jane", user.FirstName.String)
		assert.Equal(t, "Smith", user.LastName.String)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})

	t.Run("User Not Found", func(t *testing.T) {
		userID := uuid.New()

		mock.ExpectQuery(`SELECT (.+) FROM users WHERE id`).
			WithArgs(userID).
			WillReturnError(sql.ErrNoRows)

		user, err := repo.GetUserByID(userID)
		require.NoError(t, err)
		assert.Nil(t, user)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})
}

func TestUpdateProfile(t *testing.T) {
	repo, mock, cleanup := setupUserRepoTest(t)
	defer cleanup()

	t.Run("Success", func(t *testing.T) {
		userID := uuid.New()
		firstName := "John"
		lastName := "Doe"
		email := "john.doe@example.com"
		address := "123 Main Street"
		city := "Colombo"
		postalCode := "10100"

		mock.ExpectExec(`UPDATE users SET`).
			WithArgs(firstName, lastName, email, address, city, postalCode, sqlmock.AnyArg(), userID).
			WillReturnResult(sqlmock.NewResult(1, 1))

		err := repo.UpdateProfile(userID, firstName, lastName, email, address, city, postalCode)
		require.NoError(t, err)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})

	t.Run("User Not Found", func(t *testing.T) {
		userID := uuid.New()

		mock.ExpectExec(`UPDATE users SET`).
			WithArgs("John", "Doe", "john@example.com", "123 Main St", "Colombo", "10100", sqlmock.AnyArg(), userID).
			WillReturnResult(sqlmock.NewResult(0, 0))

		err := repo.UpdateProfile(userID, "John", "Doe", "john@example.com", "123 Main St", "Colombo", "10100")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "user not found")

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})

	t.Run("Database Error", func(t *testing.T) {
		userID := uuid.New()

		mock.ExpectExec(`UPDATE users SET`).
			WithArgs("John", "Doe", "john@example.com", "123 Main St", "Colombo", "10100", sqlmock.AnyArg(), userID).
			WillReturnError(fmt.Errorf("database error"))

		err := repo.UpdateProfile(userID, "John", "Doe", "john@example.com", "123 Main St", "Colombo", "10100")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "failed to update profile")

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})
}

func TestIsProfileComplete(t *testing.T) {
	repo, mock, cleanup := setupUserRepoTest(t)
	defer cleanup()

	t.Run("Complete Profile", func(t *testing.T) {
		userID := uuid.New()

		mock.ExpectQuery(`SELECT profile_completed FROM users WHERE id`).
			WithArgs(userID).
			WillReturnRows(sqlmock.NewRows([]string{"profile_completed"}).AddRow(true))

		isComplete, err := repo.IsProfileComplete(userID)
		require.NoError(t, err)
		assert.True(t, isComplete)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})

	t.Run("Incomplete Profile", func(t *testing.T) {
		userID := uuid.New()

		mock.ExpectQuery(`SELECT profile_completed FROM users WHERE id`).
			WithArgs(userID).
			WillReturnRows(sqlmock.NewRows([]string{"profile_completed"}).AddRow(false))

		isComplete, err := repo.IsProfileComplete(userID)
		require.NoError(t, err)
		assert.False(t, isComplete)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})

	t.Run("User Not Found", func(t *testing.T) {
		userID := uuid.New()

		mock.ExpectQuery(`SELECT profile_completed FROM users WHERE id`).
			WithArgs(userID).
			WillReturnError(sql.ErrNoRows)

		isComplete, err := repo.IsProfileComplete(userID)
		assert.Error(t, err)
		assert.False(t, isComplete)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})
}

func TestUpdateProfileCompletion(t *testing.T) {
	repo, mock, cleanup := setupUserRepoTest(t)
	defer cleanup()

	t.Run("Mark Complete", func(t *testing.T) {
		userID := uuid.New()

		mock.ExpectQuery(`SELECT first_name, last_name, email, address FROM users WHERE id`).
			WithArgs(userID).
			WillReturnRows(sqlmock.NewRows([]string{"first_name", "last_name", "email", "address"}).
				AddRow("John", "Doe", "john@example.com", "123 Main St"))

		mock.ExpectExec(`UPDATE users SET profile_completed`).
			WithArgs(true, sqlmock.AnyArg(), userID).
			WillReturnResult(sqlmock.NewResult(1, 1))

		err := repo.UpdateProfileCompletion(userID)
		require.NoError(t, err)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})

	t.Run("Mark Incomplete - Missing Fields", func(t *testing.T) {
		userID := uuid.New()

		mock.ExpectQuery(`SELECT first_name, last_name, email, address FROM users WHERE id`).
			WithArgs(userID).
			WillReturnRows(sqlmock.NewRows([]string{"first_name", "last_name", "email", "address"}).
				AddRow("John", "Doe", "", "123 Main St"))

		mock.ExpectExec(`UPDATE users SET profile_completed`).
			WithArgs(false, sqlmock.AnyArg(), userID).
			WillReturnResult(sqlmock.NewResult(1, 1))

		err := repo.UpdateProfileCompletion(userID)
		require.NoError(t, err)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})

	t.Run("User Not Found", func(t *testing.T) {
		userID := uuid.New()

		mock.ExpectQuery(`SELECT first_name, last_name, email, address FROM users WHERE id`).
			WithArgs(userID).
			WillReturnError(sql.ErrNoRows)

		err := repo.UpdateProfileCompletion(userID)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "user not found")

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})
}

func TestGetOrCreateUser(t *testing.T) {
	repo, mock, cleanup := setupUserRepoTest(t)
	defer cleanup()

	t.Run("Get Existing User", func(t *testing.T) {
		phone := "+94712345678"
		userID := uuid.New()
		now := time.Now()

		mock.ExpectQuery(`SELECT (.+) FROM users WHERE phone`).
			WithArgs(phone).
			WillReturnRows(sqlmock.NewRows([]string{
				"id", "phone", "first_name", "last_name", "email", "address",
				"city", "postal_code", "roles", "status", "profile_completed",
				"phone_verified", "email_verified", "created_at", "updated_at",
			}).AddRow(
				userID, phone, "John", "Doe", "john@example.com", "123 Main St",
				"Colombo", "10100", []byte(`{"passenger"}`), "active", true,
				true, true, now, now,
			))

		user, isNew, err := repo.GetOrCreateUser(phone)
		require.NoError(t, err)
		assert.NotNil(t, user)
		assert.False(t, isNew)
		assert.Equal(t, phone, user.Phone)
		assert.Equal(t, "John", user.FirstName.String)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})

	t.Run("Create New User", func(t *testing.T) {
		phone := "+94712345678"

		mock.ExpectQuery(`SELECT (.+) FROM users WHERE phone`).
			WithArgs(phone).
			WillReturnError(sql.ErrNoRows)

		mock.ExpectExec(`INSERT INTO users`).
			WithArgs(
				sqlmock.AnyArg(), phone, sqlmock.AnyArg(), "active",
				false, true, false, sqlmock.AnyArg(), sqlmock.AnyArg(),
			).
			WillReturnResult(sqlmock.NewResult(1, 1))

		user, isNew, err := repo.GetOrCreateUser(phone)
		require.NoError(t, err)
		assert.NotNil(t, user)
		assert.True(t, isNew)
		assert.Equal(t, phone, user.Phone)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})
}

func TestUpdateUserStatus(t *testing.T) {
	repo, mock, cleanup := setupUserRepoTest(t)
	defer cleanup()

	t.Run("Update to Active", func(t *testing.T) {
		userID := uuid.New()

		mock.ExpectExec(`UPDATE users SET status`).
			WithArgs("active", sqlmock.AnyArg(), userID).
			WillReturnResult(sqlmock.NewResult(1, 1))

		err := repo.UpdateUserStatus(userID, "active")
		require.NoError(t, err)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})

	t.Run("Update to Suspended", func(t *testing.T) {
		userID := uuid.New()

		mock.ExpectExec(`UPDATE users SET status`).
			WithArgs("suspended", sqlmock.AnyArg(), userID).
			WillReturnResult(sqlmock.NewResult(1, 1))

		err := repo.UpdateUserStatus(userID, "suspended")
		require.NoError(t, err)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})

	t.Run("User Not Found", func(t *testing.T) {
		userID := uuid.New()

		mock.ExpectExec(`UPDATE users SET status`).
			WithArgs("banned", sqlmock.AnyArg(), userID).
			WillReturnResult(sqlmock.NewResult(0, 0))

		err := repo.UpdateUserStatus(userID, "banned")
		assert.Error(t, err)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})
}

func TestAddUserRole(t *testing.T) {
	repo, mock, cleanup := setupUserRepoTest(t)
	defer cleanup()

	t.Run("Success", func(t *testing.T) {
		userID := uuid.New()
		role := "driver"

		mock.ExpectExec(`UPDATE users SET roles`).
			WithArgs(role, sqlmock.AnyArg(), userID).
			WillReturnResult(sqlmock.NewResult(1, 1))

		err := repo.AddUserRole(userID, role)
		require.NoError(t, err)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})

	t.Run("Invalid Role", func(t *testing.T) {
		userID := uuid.New()

		err := repo.AddUserRole(userID, "invalid_role")
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "invalid role")
	})

	t.Run("Database Error", func(t *testing.T) {
		userID := uuid.New()

		mock.ExpectExec(`UPDATE users SET roles`).
			WithArgs("admin", sqlmock.AnyArg(), userID).
			WillReturnError(fmt.Errorf("database error"))

		err := repo.AddUserRole(userID, "admin")
		assert.Error(t, err)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})
}

func TestRemoveUserRole(t *testing.T) {
	repo, mock, cleanup := setupUserRepoTest(t)
	defer cleanup()

	t.Run("Success", func(t *testing.T) {
		userID := uuid.New()
		role := "driver"

		mock.ExpectExec(`UPDATE users SET roles`).
			WithArgs(role, sqlmock.AnyArg(), userID).
			WillReturnResult(sqlmock.NewResult(1, 1))

		err := repo.RemoveUserRole(userID, role)
		require.NoError(t, err)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})

	t.Run("Database Error", func(t *testing.T) {
		userID := uuid.New()

		mock.ExpectExec(`UPDATE users SET roles`).
			WithArgs("passenger", sqlmock.AnyArg(), userID).
			WillReturnError(fmt.Errorf("database error"))

		err := repo.RemoveUserRole(userID, "passenger")
		assert.Error(t, err)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})
}

func TestListUsers(t *testing.T) {
	repo, mock, cleanup := setupUserRepoTest(t)
	defer cleanup()

	t.Run("Success", func(t *testing.T) {
		now := time.Now()
		user1ID := uuid.New()
		user2ID := uuid.New()

		mock.ExpectQuery(`SELECT (.+) FROM users ORDER BY created_at DESC LIMIT`).
			WithArgs(10, 0).
			WillReturnRows(sqlmock.NewRows([]string{
				"id", "phone", "first_name", "last_name", "email", "address",
				"city", "postal_code", "roles", "status", "profile_completed",
				"phone_verified", "email_verified", "created_at", "updated_at",
			}).
				AddRow(user1ID, "+94712345678", "John", "Doe", "john@example.com", "123 Main St",
					"Colombo", "10100", []byte(`{"passenger"}`), "active", true, true, true, now, now).
				AddRow(user2ID, "+94723456789", "Jane", "Smith", "jane@example.com", "456 Oak Ave",
					"Kandy", "20000", []byte(`{"passenger","driver"}`), "active", true, true, false, now, now))

		users, err := repo.ListUsers(10, 0)
		require.NoError(t, err)
		assert.Len(t, users, 2)
		assert.Equal(t, "John", users[0].FirstName.String)
		assert.Equal(t, "Jane", users[1].FirstName.String)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})

	t.Run("Empty Result", func(t *testing.T) {
		mock.ExpectQuery(`SELECT (.+) FROM users ORDER BY created_at DESC LIMIT`).
			WithArgs(10, 0).
			WillReturnRows(sqlmock.NewRows([]string{
				"id", "phone", "first_name", "last_name", "email", "address",
				"city", "postal_code", "roles", "status", "profile_completed",
				"phone_verified", "email_verified", "created_at", "updated_at",
			}))

		users, err := repo.ListUsers(10, 0)
		require.NoError(t, err)
		assert.Len(t, users, 0)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})

	t.Run("Database Error", func(t *testing.T) {
		mock.ExpectQuery(`SELECT (.+) FROM users ORDER BY created_at DESC LIMIT`).
			WithArgs(10, 0).
			WillReturnError(fmt.Errorf("database error"))

		users, err := repo.ListUsers(10, 0)
		assert.Error(t, err)
		assert.Nil(t, users)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})
}

func TestCountUsers(t *testing.T) {
	repo, mock, cleanup := setupUserRepoTest(t)
	defer cleanup()

	t.Run("Success", func(t *testing.T) {
		mock.ExpectQuery(`SELECT COUNT\(\*\) FROM users`).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(42))

		count, err := repo.CountUsers()
		require.NoError(t, err)
		assert.Equal(t, 42, count)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})

	t.Run("Zero Users", func(t *testing.T) {
		mock.ExpectQuery(`SELECT COUNT\(\*\) FROM users`).
			WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(0))

		count, err := repo.CountUsers()
		require.NoError(t, err)
		assert.Equal(t, 0, count)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})

	t.Run("Database Error", func(t *testing.T) {
		mock.ExpectQuery(`SELECT COUNT\(\*\) FROM users`).
			WillReturnError(fmt.Errorf("database error"))

		count, err := repo.CountUsers()
		assert.Error(t, err)
		assert.Equal(t, 0, count)

		err = mock.ExpectationsWereMet()
		assert.NoError(t, err)
	})
}
