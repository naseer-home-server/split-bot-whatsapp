package server

import (
	"errors"
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
	"gorm.io/gorm"

	"github.com/naseer2426/split-bot-whatsapp/internal/db"
	"github.com/naseer2426/split-bot-whatsapp/internal/totals"
)

type totalsCreateRequest struct {
	GroupID     string            `json:"group_id" binding:"required"`
	Title       string            `json:"title"`
	Items       []totals.LineItem `json:"items" binding:"required,min=1"`
	Tax         []totals.Tax      `json:"tax"`
	Discount    float64           `json:"discount"`
	TotalInBill float64           `json:"total_in_bill"`
}

type totalsAssignmentsRequest struct {
	TotalsID    int                 `json:"totals_id" binding:"required"`
	Assignments map[string][]string `json:"assignments" binding:"required"`
}

func (s *Server) totalsCreateHandler(c *gin.Context) {
	var req totalsCreateRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request body",
			"details": err.Error(),
		})
		return
	}

	row, prepared, err := s.handler.CreateBillTotals(c.Request.Context(), req.GroupID, req.Title, req.Items, req.Tax, req.Discount, req.TotalInBill)
	if err != nil {
		var mismatch *totals.MismatchError
		if errors.As(err, &mismatch) {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":            "totals mismatch",
				"details":          mismatch.Error(),
				"item_total":       mismatch.ItemTotal,
				"calculated_total": mismatch.CalculatedTotal,
				"total_in_bill":    mismatch.TotalInBill,
				"difference":       mismatch.Difference,
			})
			return
		}
		status := http.StatusInternalServerError
		errLabel := "Failed to create totals"
		if isTotalsCreateClientError(err) {
			status = http.StatusBadRequest
			errLabel = "Invalid totals request"
		}
		c.JSON(status, gin.H{
			"error":   errLabel,
			"details": err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"status":           "success",
		"totals_id":        row.ID,
		"poll_id":          row.PollID,
		"item_total":       prepared.ItemTotal,
		"calculated_total": prepared.CalculatedTotal,
		"total_in_bill":    prepared.TotalInBill,
		"difference":       prepared.Difference,
	})
}

func (s *Server) totalsAssignmentsGetHandler(c *gin.Context) {
	idStr := c.Query("totals_id")
	totalsID, err := strconv.Atoi(idStr)
	if err != nil || totalsID <= 0 {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid or missing totals_id query parameter",
			"details": "totals_id must be a positive integer",
		})
		return
	}

	row, snap, err := s.handler.GetBillAssignments(c.Request.Context(), totalsID)
	if err != nil {
		writeTotalsLoadError(c, err)
		return
	}
	c.JSON(http.StatusOK, totalsSnapshotResponse(row, snap))
}

func (s *Server) totalsAssignmentsPutHandler(c *gin.Context) {
	var req totalsAssignmentsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{
			"error":   "Invalid request body",
			"details": err.Error(),
		})
		return
	}

	row, snap, err := s.handler.SetBillAssignments(c.Request.Context(), req.TotalsID, req.Assignments)
	if err != nil {
		if isClientAssignmentsError(err) {
			c.JSON(http.StatusBadRequest, gin.H{
				"error":   "Invalid assignments",
				"details": err.Error(),
			})
			return
		}
		writeTotalsLoadError(c, err)
		return
	}
	c.JSON(http.StatusOK, totalsSnapshotResponse(row, snap))
}

func totalsSnapshotResponse(row *db.SplitbotTotals, snap totals.Snapshot) gin.H {
	return gin.H{
		"status":           "success",
		"totals_id":        row.ID,
		"poll_id":          row.PollID,
		"group_id":         row.GroupID,
		"items":            snap.Items,
		"tax":              snap.Tax,
		"discount":         snap.Discount,
		"item_total":       snap.ItemTotal,
		"total_in_bill":    snap.TotalInBill,
		"calculated_total": snap.CalculatedTotal,
		"assignments":      snap.Assignments,
		"unassigned":       snap.Unassigned,
		"shares":           snap.Shares,
		"unassigned_owed":  snap.UnassignedOwed,
	}
}

func writeTotalsLoadError(c *gin.Context, err error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		c.JSON(http.StatusNotFound, gin.H{
			"error":   "Totals not found",
			"details": err.Error(),
		})
		return
	}
	c.JSON(http.StatusInternalServerError, gin.H{
		"error":   "Failed to load totals",
		"details": err.Error(),
	})
}

func isClientAssignmentsError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.HasPrefix(msg, "unknown item id") || strings.HasPrefix(msg, "item ")
}

func isTotalsCreateClientError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	switch {
	case strings.HasPrefix(msg, "create poll:"),
		strings.HasPrefix(msg, "create totals row:"),
		strings.HasPrefix(msg, "save totals poll_id:"),
		strings.HasPrefix(msg, "marshal "):
		return false
	default:
		return true
	}
}
