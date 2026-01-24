package tests

import (
	"encoding/json"
	"testing"

	"github.com/developia-II/ecommerce-backend/internal/models"
	"github.com/stretchr/testify/assert"
)

func TestJSON_Marshall_User_Struct(t *testing.T) {
	user := models.User{
		Name:     "favour opia",
		Email:    "favour@gmail.com",
		Phone:    "07027262819",
		Password: "favour1234",
		Role:     "buyer",
	}

	jsonData, err := json.Marshal(user)
	assert.NoError(t, err)

	jsonString := string(jsonData)
	assert.Contains(t, jsonString, "favour@gmail.com")
	assert.NotContains(t, jsonString, "favour1234")
}

func TestJSON_UNMarshall_User_Struct(t *testing.T) {
	jsonData := `{"email":"favour@gmail.com", "name":"favour", "password":"secure123"}`
	var user models.User
	err := json.Unmarshal([]byte(jsonData), &user)
	assert.NoError(t, err)

	assert.Equal(t, "favour@gmail.com", user.Email)
	assert.Equal(t, "favour", user.Name)

}
