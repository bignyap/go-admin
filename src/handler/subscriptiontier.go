package handler

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/bignyap/go-gate-keeper/database/dbutils"
	"github.com/bignyap/go-gate-keeper/database/sqlcgen"
	"github.com/bignyap/go-gate-keeper/utils/converter"
	"github.com/bignyap/go-gate-keeper/utils/formvalidator"
	"github.com/bignyap/go-gate-keeper/utils/misc"
)

type CreateSubTierParams struct {
	Name        string    `json:"name"`
	Description *string   `json:"description"`
	CreatedAt   time.Time `json:"created_at"`
	UpdatedAt   time.Time `json:"updated_at"`
}

type CreateSubTierOuput struct {
	ID int `json:"id"`
	CreateSubTierParams
}

func CreateSubcriptionTierFormValidation(r *http.Request) (*sqlcgen.CreateSubscriptionTierParams, error) {

	err := formvalidator.ParseFormData(r)
	if err != nil {
		return nil, err
	}

	strFields := []string{"name"}
	strParsed, err := formvalidator.ParseStringFromForm(r, strFields)
	if err != nil {
		return nil, err
	}

	nullStrField := []string{"description"}
	nullStrParsed, err := formvalidator.ParseNullStringFromForm(r, nullStrField)
	if err != nil {
		return nil, err
	}

	input := sqlcgen.CreateSubscriptionTierParams{
		TierName:        strParsed["name"],
		TierDescription: nullStrParsed["description"],
		TierCreatedAt:   int32(misc.ToUnixTime()),
		TierUpdatedAt:   int32(misc.ToUnixTime()),
	}

	return &input, nil
}

func CreateSubscriptionTierJSONValidation(r *http.Request) ([]sqlcgen.CreateSubscriptionTiersParams, error) {

	var inputs []CreateSubTierParams

	decoder := json.NewDecoder(r.Body)
	err := decoder.Decode(&inputs)
	if err != nil {
		return nil, err
	}

	var outputs []sqlcgen.CreateSubscriptionTiersParams

	currentTime := int32(misc.ToUnixTime())

	for _, input := range inputs {
		batchInput := sqlcgen.CreateSubscriptionTiersParams{
			TierName:        input.Name,
			TierDescription: converter.StrToNullStr(*input.Description),
			TierCreatedAt:   currentTime,
			TierUpdatedAt:   currentTime,
		}
		outputs = append(outputs, batchInput)
	}

	return outputs, nil
}

type BulkCreateSubscriptionTierInserter struct {
	SubscriptionTiers []sqlcgen.CreateSubscriptionTiersParams
	ApiConfig         *ApiConfig
}

func (input BulkCreateSubscriptionTierInserter) InsertRows(ctx context.Context, tx *sql.Tx) (int64, error) {

	affectedRows, err := input.ApiConfig.DB.CreateSubscriptionTiers(ctx, input.SubscriptionTiers)
	if err != nil {
		return 0, err
	}

	return affectedRows, nil
}

func (apiCfg *ApiConfig) CreateSubscriptionTierInBatchHandler(w http.ResponseWriter, r *http.Request) {

	input, err := CreateSubscriptionTierJSONValidation(r)
	if err != nil {
		respondWithError(w, StatusBadRequest, err.Error())
		return
	}

	inserter := BulkCreateSubscriptionTierInserter{
		SubscriptionTiers: input,
		ApiConfig:         apiCfg,
	}

	affectedRows, err := dbutils.InsertWithTransaction(r.Context(), apiCfg.Conn, inserter)
	if err != nil {
		respondWithError(w, StatusBadRequest, fmt.Sprintf("couldn't create the subscription tiers: %s", err))
		return
	}

	respondWithJSON(w, StatusCreated, map[string]int64{"affected_rows": affectedRows})
}

func (apiCfg *ApiConfig) CreateSubcriptionTierHandler(w http.ResponseWriter, r *http.Request) {

	input, err := CreateSubcriptionTierFormValidation(r)
	if err != nil {
		respondWithError(w, StatusBadRequest, err.Error())
		return
	}

	subTier, err := apiCfg.DB.CreateSubscriptionTier(r.Context(), *input)
	if err != nil {
		respondWithError(w, StatusBadRequest, fmt.Sprintf("couldn't create the subscription tier: %s", err))
		return
	}

	insertedID, err := subTier.LastInsertId()
	if err != nil {
		respondWithError(w, StatusInternalServerError, fmt.Sprintf("couldn't retrieve last insert ID: %s", err))
		return
	}

	var description *string
	if input.TierDescription.Valid {
		description = &input.TierDescription.String
	}

	output := CreateSubTierOuput{
		ID: int(insertedID),
		CreateSubTierParams: CreateSubTierParams{
			Name:        input.TierName,
			Description: description,
			CreatedAt:   misc.FromUnixTime32(input.TierCreatedAt),
			UpdatedAt:   misc.FromUnixTime32(input.TierUpdatedAt),
		},
	}

	respondWithJSON(w, StatusCreated, output)
}

func (apiCfg *ApiConfig) ListSubscriptionTiersHandler(w http.ResponseWriter, r *http.Request) {

	page, n := ExtractPaginationDetail(w, r)
	input := sqlcgen.ListSubscriptionTierParams{
		Limit:  int32(page),
		Offset: int32(n),
	}

	subTiers, err := apiCfg.DB.ListSubscriptionTier(r.Context(), input)
	if err != nil {
		respondWithError(w, StatusBadRequest, fmt.Sprintf("couldn't retrieve the subscripion tiers: %s", err))
		return
	}

	var output []CreateSubTierOuput

	for _, subTier := range subTiers {

		var description *string
		if subTier.TierDescription.Valid {
			description = &subTier.TierDescription.String
		}

		output = append(output, CreateSubTierOuput{
			ID: int(subTier.SubscriptionTierID),
			CreateSubTierParams: CreateSubTierParams{
				Name:        subTier.TierName,
				Description: description,
				CreatedAt:   misc.FromUnixTime32(subTier.TierCreatedAt),
				UpdatedAt:   misc.FromUnixTime32(subTier.TierUpdatedAt),
			},
		})
	}

	respondWithJSON(w, StatusOK, output)
}

func (apiCfg *ApiConfig) DeleteSubscriptionTierHandler(w http.ResponseWriter, r *http.Request) {

	id, err := converter.StrToInt(r.PathValue("id"))
	if err != nil {
		respondWithError(w, StatusBadRequest, "ID is required")
		return
	}

	err = apiCfg.DB.DeleteSubscriptionTierById(r.Context(), int32(id))
	if err != nil {
		respondWithError(w, StatusBadRequest, fmt.Sprintf("couldn't delete the subscription tier: %s", err))
		return
	}

	respondWithJSON(w, StatusNoContent, map[string]string{
		"message": fmt.Sprintf("Subscription tier with ID %d deleted successfully", int32(id)),
	})
}
