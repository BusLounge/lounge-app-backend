package handlers

import (
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
	"github.com/smarttransit/sms-auth-backend/internal/database"
	"github.com/smarttransit/sms-auth-backend/internal/middleware"
	"github.com/smarttransit/sms-auth-backend/internal/models"
)

// LoungeTransportLocationHandler handles HTTP requests related to lounge transport locations
type LoungeTransportLocationHandler struct {
	loungeOwnerRepo                  *database.LoungeOwnerRepository
	loungeRepo                       *database.LoungeRepository
	loungeTransportLocationRepo      *database.LoungeTransportLocationRepository
	loungeTransportLocationPriceRepo *database.LoungeTransportLocationPriceRepository
}

// NewLoungeTransportLocationHandler creates new lounge transport location handler
func NewLoungeTransportLocationHandler(
	loungeOwnerRepo *database.LoungeOwnerRepository,
	loungeRepo *database.LoungeRepository,
	loungeTransportLocationRepo *database.LoungeTransportLocationRepository,
	loungeTransportLocationPriceRepo *database.LoungeTransportLocationPriceRepository,
) *LoungeTransportLocationHandler {
	return &LoungeTransportLocationHandler{
		loungeOwnerRepo:                  loungeOwnerRepo,
		loungeRepo:                       loungeRepo,
		loungeTransportLocationRepo:      loungeTransportLocationRepo,
		loungeTransportLocationPriceRepo: loungeTransportLocationPriceRepo,
	}
}

type AddLoungeTransportLocationRequest struct {
	Location  string  `json:"location" binding:"required,min=2,max=200"`
	Latitude  float64 `json:"latitude" binding:"required,gte=-90,lte=90"`
	Longitude float64 `json:"longitude" binding:"required,gte=-180,lte=180"`
	// NEWLY ADDED ROWS TO GET LoungeID from the request itself
	LoungeID uuid.UUID `json:"lounge_id" binding:"required"`
	// Estimated Duration
	EstDuration int `json:"est_duration" binding:"required"`
	// NEW additin
	Distance float64 `json:"distance" binding:"required"`

}

type UpdateLoungeTransportLocationRequest struct {
	Location          *string                               `json:"location" binding:"omitempty,min=2,max=200"`
	Latitude          *float64                              `json:"latitude" binding:"omitempty,gte=-90,lte=90"`
	Longitude         *float64                              `json:"longitude" binding:"omitempty,gte=-180,lte=180"`
	EstDuration       *int                                  `json:"est_duration" binding:"omitempty,gte=0"`
	Distance          *float64                              `json:"distance" binding:"omitempty,gte=0"`
	Status            *models.LoungeTransportLocationStatus `json:"status" binding:"omitempty,oneof=active inactive"`
	ThreeWheelerPrice *float64                              `json:"three_wheeler_price" binding:"omitempty,gte=0"`
	CarPrice          *float64                              `json:"car_price" binding:"omitempty,gte=0"`
	VanPrice          *float64                              `json:"van_price" binding:"omitempty,gte=0"`
}

// Add transport locations to lounge
func (h *LoungeTransportLocationHandler) AddLoungeTransportLocation(c *gin.Context) {

	// creating a struct variable to store request
	var req AddLoungeTransportLocationRequest

	// getting the data fed by the user
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "Validation failed",
			Message: err.Error(),
		})
		return
	}

	// Get user context from JWT middleware
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User context not found",
		})
		return
	}

	// Get lounge owner record
	owner, err := h.loungeOwnerRepo.GetLoungeOwnerByUserID(userCtx.UserID)
	if err != nil || owner == nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "Lounge owner not found",
		})
		return
	}

	// Verify lounge ownership using LoungeID from request only
	lounge, err := h.loungeRepo.GetLoungeByID(req.LoungeID)
	if err != nil || lounge == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "Lounge not found",
		})
		return
	}

	if lounge.LoungeOwnerID != owner.ID {
		c.JSON(http.StatusForbidden, ErrorResponse{
			Error:   "forbidden",
			Message: "You don't have permission to add transport locations for this lounge",
		})
		return
	}

	// converting the data into the feeding model
	TransportLocation := &models.LoungeTransportLocation{

		LoungeID:  req.LoungeID,
		Location:  req.Location,
		Latitude:  req.Latitude,
		Longitude: req.Longitude,
		EstDuration: req.EstDuration,
		// NEW addition
		Distance: req.Distance,
	}

	// // Add transport location to lounge
	createdLocation, err := h.loungeTransportLocationRepo.AddTransportLocationToLounge(*TransportLocation)

	// checking for errors
	if err != nil {
		log.Printf("ERROR: Failed to add location \"%s\" for lounge  %s: %v", req.Location, req.LoungeID.String(), err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "add_failed",
			Message: "Failed to add location",
		})
		return

	}

	// if all good then sending statusCreated with the data struct for reference
	c.JSON(http.StatusCreated, createdLocation)

}

// get transport locations by lounge id
func (h *LoungeTransportLocationHandler) GetLoungeTransportLocationsByLoungeID(c *gin.Context) {

	// Get user context from JWT middleware
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User context not found",
		})
		return
	}

	// getting the loungeID
	loungeID := uuid.MustParse(c.Param("id"))

	// Get lounge owner record
	owner, err := h.loungeOwnerRepo.GetLoungeOwnerByUserID(userCtx.UserID)
	if err != nil || owner == nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "Lounge owner not found",
		})
		return
	}

	// Verify lounge ownership
	lounge, err := h.loungeRepo.GetLoungeByID(loungeID)
	if err != nil || lounge == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "Lounge not found",
		})
		return
	}

	if lounge.LoungeOwnerID != owner.ID {
		c.JSON(http.StatusForbidden, ErrorResponse{
			Error:   "forbidden",
			Message: "You don't have permission to get transport locations for this lounge",
		})
		return
	}

	// getting the transport locations by loungeID
	locations, err := h.loungeTransportLocationRepo.GetLoungeTransportLocationsByLoungeID(loungeID)
	if err != nil {
		log.Printf("ERROR: Failed to get transport locations for lounge %s: %v", loungeID, err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "database_error",
			Message: "Failed to retrieve locations",
		})
		return
	}

	// converting locations list inorder to send back formatting structure
	response := make([]gin.H, 0, len(locations))

	// looping and feeding the data
	for _, s := range locations {
		item := gin.H{
			"id":           s.ID,
			"lounge_id":    s.LoungeID,
			"location":     s.Location,
			"latitude":     s.Latitude,
			"longitude":    s.Longitude,
			"est_duration": s.EstDuration,
			"distance":     s.Distance,
			"status":       s.Status,
			"created_at":   s.CreatedAt,
			"updated_at":   s.UpdatedAt,
		}

		if h.loungeTransportLocationPriceRepo != nil {
			priceData, _ := h.loungeTransportLocationPriceRepo.GetLoungeTransportLocationPrices(loungeID, s.ID)
			if priceData != nil {
				item["prices"] = map[string]float64{
					"Three Wheeler":       priceData.ThreeWheelerPrice,
					"Car":                 priceData.CarPrice,
					"Van":                 priceData.VanPrice,
					"three_wheeler_price": priceData.ThreeWheelerPrice,
					"car_price":           priceData.CarPrice,
					"van_price":           priceData.VanPrice,
				}
			}
		}

		response = append(response, item)
	}

	// sending back the dataset with the length
	c.JSON(http.StatusOK, gin.H{
		"transport_locations": response,
		"total":               len(response),
	})

}

// delete lounge transport locations by location id
func (h *LoungeTransportLocationHandler) DeleteLoungeTransportLocationByID(c *gin.Context) {

	// Get user context from JWT middleware
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User context not found",
		})
		return
	}

	// getting the loungeID
	loungeID := uuid.MustParse(c.Param("id"))
	// getting the locationID
	locationID := uuid.MustParse(c.Param("location_id"))

	// Get lounge owner record
	owner, err := h.loungeOwnerRepo.GetLoungeOwnerByUserID(userCtx.UserID)
	if err != nil || owner == nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "Lounge owner not found",
		})
		return
	}

	// Verify lounge ownership
	lounge, err := h.loungeRepo.GetLoungeByID(loungeID)
	if err != nil || lounge == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "Lounge not found",
		})
		return
	}

	if lounge.LoungeOwnerID != owner.ID {
		c.JSON(http.StatusForbidden, ErrorResponse{
			Error:   "forbidden",
			Message: "You don't have permission to delete transport locations for this lounge",
		})
		return
	}

	// verifying transport location belong to the lounge
	locationData, err := h.loungeTransportLocationRepo.GetLoungeTransportLocationByID(locationID)
	if err != nil {
		log.Printf("ERROR: Failed to get location %s: %v", locationID, err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "database_error",
			Message: "Failed to get location",
		})
		return
	}
	if locationData == nil {
		log.Printf("ERROR: No location found %s:", locationID)
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "location not found",
		})
		return
	}

	// verify if the transport location actually belongs to the lounge
	if locationData.LoungeID != loungeID {
		log.Printf("ERROR:Location does not belong to this lounge %s:", loungeID)
		c.JSON(http.StatusForbidden, ErrorResponse{
			Error:   "forbidden",
			Message: "Location does not belong to this lounge",
		})
		return
	}

	// delete lounge transport location by id
	delErr := h.loungeTransportLocationRepo.DeleteLoungeTransportLocationByID(locationID)
	if delErr != nil {
		log.Printf("ERROR: Failed to remove location %s: %v", locationID, delErr)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "delete_failed",
			Message: "failed to delete transport location",
		})
		return
	}

	// success message
	c.Status(http.StatusNoContent)

}

// update lounge transport location by location id
func (h *LoungeTransportLocationHandler) UpdateLoungeTransportLocationByID(c *gin.Context) {

	// validating the user who is sending the request
	userCtx, exists := middleware.GetUserContext(c)
	if !exists {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "User context not found",
		})
		return
	}

	// getting the loungeID
	loungeID := uuid.MustParse(c.Param("id"))
	// getting the locationID
	locationID := uuid.MustParse(c.Param("location_id"))

	// extracting the lounge owner
	owner, err := h.loungeOwnerRepo.GetLoungeOwnerByUserID(userCtx.UserID)
	if err != nil {
		log.Printf("ERROR: Failed to get owner for user %s: %v", userCtx.UserID, err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "database_error",
			Message: "Failed to verify ownership",
		})
		return
	}
	if owner == nil {
		c.JSON(http.StatusUnauthorized, ErrorResponse{
			Error:   "unauthorized",
			Message: "Lounge owner not found",
		})
		return
	}

	//verifying the lounge ownership
	lounge, err := h.loungeRepo.GetLoungeByID(loungeID)
	if err != nil {
		log.Printf("ERROR: Failed to get lounge %s: %v", loungeID, err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "database_error",
			Message: "Failed to verify lounge",
		})
		return
	}
	if lounge == nil {
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "Lounge not found",
		})
		return
	}
	if lounge.LoungeOwnerID != owner.ID {
		c.JSON(http.StatusForbidden, ErrorResponse{
			Error:   "forbidden",
			Message: "You don't have permission to update locations in this lounge",
		})
		return
	}

	// verifying transport location belong to the lounge
	locationData, err := h.loungeTransportLocationRepo.GetLoungeTransportLocationByID(locationID)
	if err != nil {
		log.Printf("ERROR: Failed to get location %s: %v", locationID, err)
		c.JSON(http.StatusInternalServerError, ErrorResponse{
			Error:   "database_error",
			Message: "Failed to get location",
		})
		return
	}
	if locationData == nil {
		log.Printf("ERROR: No location found %s:", locationID)
		c.JSON(http.StatusNotFound, ErrorResponse{
			Error:   "not_found",
			Message: "location not found",
		})
		return
	}

	// verify if the transport location actually belongs to the lounge
	if locationData.LoungeID != loungeID {
		log.Printf("ERROR:Location does not belong to this lounge %s:", loungeID)
		c.JSON(http.StatusForbidden, ErrorResponse{
			Error:   "forbidden",
			Message: "Location does not belong to this lounge",
		})
		return
	}

	// creating a update request struct for location
	var req UpdateLoungeTransportLocationRequest

	// passing the data into the request struct
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, ErrorResponse{
			Error:   "validation_failed",
			Message: err.Error(),
		})
		return
	}

	// creating a map to convert data into the required format
	updates := make(map[string]interface{})

	if req.Location != nil {
		updates["location"] = *req.Location
	}
	if req.Latitude != nil {
		updates["latitude"] = *req.Latitude
	}
	if req.Longitude != nil {
		updates["longitude"] = *req.Longitude
	}
	if req.EstDuration != nil {
		updates["est_duration"] = *req.EstDuration
	}
	if req.Distance != nil {
		updates["distance"] = *req.Distance
	}
	if req.Status != nil {
		updates["status"] = *req.Status
	}

	// update the location
	if len(updates) > 0 {
		if err := h.loungeTransportLocationRepo.UpdateLoungeTransportLocationByID(locationID, updates); err != nil {
			// if error occured returning the server error
			c.JSON(http.StatusInternalServerError, ErrorResponse{
				Error:   "update_failed",
				Message: "Failed to update location",
			})
			return
		}
	}

	if h.loungeTransportLocationPriceRepo != nil && (req.ThreeWheelerPrice != nil || req.CarPrice != nil || req.VanPrice != nil) {
		currentPrices, _ := h.loungeTransportLocationPriceRepo.GetLoungeTransportLocationPrices(loungeID, locationID)
		tw := 0.0
		car := 0.0
		van := 0.0
		if currentPrices != nil {
			tw = currentPrices.ThreeWheelerPrice
			car = currentPrices.CarPrice
			van = currentPrices.VanPrice
		}
		if req.ThreeWheelerPrice != nil {
			tw = *req.ThreeWheelerPrice
		}
		if req.CarPrice != nil {
			car = *req.CarPrice
		}
		if req.VanPrice != nil {
			van = *req.VanPrice
		}
		_, err := h.loungeTransportLocationPriceRepo.SetLoungeTransportLocationPrices(loungeID, locationID, tw, car, van)
		if err != nil {
			log.Printf("ERROR: Failed to update prices for location %s: %v", locationID, err)
		}
	}

	// if ok returning the update location
	updatedLocation, _ := h.loungeTransportLocationRepo.GetLoungeTransportLocationByID(locationID)
	if updatedLocation == nil {
		c.JSON(http.StatusOK, gin.H{"message": "Location updated successfully"})
		return
	}

	res := gin.H{
		"id":           updatedLocation.ID,
		"lounge_id":    updatedLocation.LoungeID,
		"location":     updatedLocation.Location,
		"latitude":     updatedLocation.Latitude,
		"longitude":    updatedLocation.Longitude,
		"est_duration": updatedLocation.EstDuration,
		"distance":     updatedLocation.Distance,
		"status":       updatedLocation.Status,
		"created_at":   updatedLocation.CreatedAt,
		"updated_at":   updatedLocation.UpdatedAt,
	}

	if h.loungeTransportLocationPriceRepo != nil {
		priceData, _ := h.loungeTransportLocationPriceRepo.GetLoungeTransportLocationPrices(loungeID, locationID)
		if priceData != nil {
			res["prices"] = map[string]float64{
				"Three Wheeler":       priceData.ThreeWheelerPrice,
				"Car":                 priceData.CarPrice,
				"Van":                 priceData.VanPrice,
				"three_wheeler_price": priceData.ThreeWheelerPrice,
				"car_price":           priceData.CarPrice,
				"van_price":           priceData.VanPrice,
			}
		}
	}

	c.JSON(http.StatusOK, res)

}
