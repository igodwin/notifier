package grpc

import (
	"context"
	"fmt"

	"github.com/google/uuid"
	pb "github.com/igodwin/notifier/api/grpc/pb"
	"github.com/igodwin/notifier/internal/domain"
	"github.com/igodwin/notifier/internal/logging"
	"google.golang.org/protobuf/types/known/timestamppb"
)

// NotifierHandler implements the gRPC NotifierService
type NotifierHandler struct {
	pb.UnimplementedNotifierServiceServer
	service domain.NotificationService
	logger  *logging.Logger
}

// NewNotifierHandler creates a new gRPC handler
func NewNotifierHandler(svc domain.NotificationService, logger *logging.Logger) *NotifierHandler {
	return &NotifierHandler{
		service: svc,
		logger:  logger,
	}
}

// HealthCheck verifies the service is operational
func (h *NotifierHandler) HealthCheck(ctx context.Context, req *pb.HealthCheckRequest) (*pb.HealthCheckResponse, error) {
	// TODO: Implement proper health check logic
	return &pb.HealthCheckResponse{
		Healthy: true,
		Status:  "ok",
		Components: map[string]string{
			"service": "running",
		},
	}, nil
}

// SendNotification sends a single notification
func (h *NotifierHandler) SendNotification(ctx context.Context, req *pb.SendNotificationRequest) (*pb.SendNotificationResponse, error) {
	// Log incoming request
	h.logger.Infof("gRPC: Received notification request - type=%s, account=%s, recipients=%d, subject=%s",
		req.Type, req.Account, len(req.Recipients), req.Subject)

	// Convert proto notification type to domain type
	notifType := convertProtoTypeToDomain(req.Type)

	// Set default max retries if not specified
	maxRetries := int(req.MaxRetries)
	if maxRetries == 0 {
		maxRetries = 3 // Default
	}

	// Convert content type, defaulting to text
	contentType := convertProtoContentTypeToDomain(req.ContentType)

	// Build notification
	notification := &domain.Notification{
		ID:          uuid.New().String(),
		Type:        notifType,
		Account:     req.Account,
		Priority:    domain.Priority(req.Priority),
		Subject:     req.Subject,
		Body:        req.Body,
		ContentType: contentType,
		Recipients:  req.Recipients,
		CC:          req.Cc,
		BCC:         req.Bcc,
		Metadata:    convertStringMapToInterface(req.Metadata),
		MaxRetries:  maxRetries,
	}

	if req.ScheduledFor != nil {
		scheduledTime := req.ScheduledFor.AsTime()
		notification.ScheduledFor = &scheduledTime
	}

	// Send notification
	result, err := h.service.Send(ctx, notification)
	if err != nil {
		h.logger.Errorf("gRPC: Failed to send notification - type=%s, account=%s, error=%v",
			req.Type, req.Account, err)
		return &pb.SendNotificationResponse{
			Result: &pb.NotificationResult{
				Success: false,
				Error:   err.Error(),
			},
		}, nil
	}

	// Log success
	h.logger.Infof("gRPC: Notification queued successfully - id=%s, type=%s, recipients=%d",
		result.NotificationID, req.Type, len(req.Recipients))

	// Convert result to proto
	return &pb.SendNotificationResponse{
		Result: &pb.NotificationResult{
			NotificationId: result.NotificationID,
			Success:        result.Success,
			Message:        result.Message,
			SentAt:         timestamppb.New(result.SentAt),
		},
	}, nil
}

// SendBatchNotifications sends multiple notifications
func (h *NotifierHandler) SendBatchNotifications(ctx context.Context, req *pb.SendBatchNotificationsRequest) (*pb.SendBatchNotificationsResponse, error) {
	h.logger.Infof("gRPC: Received batch notification request - count=%d", len(req.Notifications))

	var results []*pb.NotificationResult
	successCount := 0

	for _, notifReq := range req.Notifications {
		resp, err := h.SendNotification(ctx, notifReq)
		if err != nil {
			results = append(results, &pb.NotificationResult{
				Success: false,
				Error:   err.Error(),
			})
		} else {
			results = append(results, resp.Result)
			if resp.Result.Success {
				successCount++
			}
		}
	}

	h.logger.Infof("gRPC: Batch notification completed - total=%d, successful=%d, failed=%d",
		len(req.Notifications), successCount, len(req.Notifications)-successCount)

	return &pb.SendBatchNotificationsResponse{
		Results: results,
	}, nil
}

// GetNotification retrieves a notification by ID
func (h *NotifierHandler) GetNotification(ctx context.Context, req *pb.GetNotificationRequest) (*pb.GetNotificationResponse, error) {
	notification, err := h.service.GetNotification(ctx, req.Id)
	if err != nil {
		return nil, err
	}

	return &pb.GetNotificationResponse{
		Notification: convertDomainToProtoNotification(notification),
	}, nil
}

// ListNotifications retrieves notifications matching a filter
func (h *NotifierHandler) ListNotifications(ctx context.Context, req *pb.ListNotificationsRequest) (*pb.ListNotificationsResponse, error) {
	// Convert proto filter to domain filter
	filter := convertProtoFilterToDomain(req.Filter)

	notifications, err := h.service.ListNotifications(ctx, filter)
	if err != nil {
		return nil, err
	}

	protoNotifications := make([]*pb.Notification, len(notifications))
	for i, notif := range notifications {
		protoNotifications[i] = convertDomainToProtoNotification(notif)
	}

	return &pb.ListNotificationsResponse{
		Notifications: protoNotifications,
		Total:         int64(len(notifications)),
	}, nil
}

// CancelNotification cancels a pending notification
func (h *NotifierHandler) CancelNotification(ctx context.Context, req *pb.CancelNotificationRequest) (*pb.CancelNotificationResponse, error) {
	err := h.service.CancelNotification(ctx, req.Id)
	if err != nil {
		return &pb.CancelNotificationResponse{
			Success: false,
			Message: err.Error(),
		}, nil
	}

	return &pb.CancelNotificationResponse{
		Success: true,
		Message: "notification cancelled successfully",
	}, nil
}

// RetryNotification retries a failed notification
func (h *NotifierHandler) RetryNotification(ctx context.Context, req *pb.RetryNotificationRequest) (*pb.RetryNotificationResponse, error) {
	result, err := h.service.RetryNotification(ctx, req.Id)
	if err != nil {
		return &pb.RetryNotificationResponse{
			Result: &pb.NotificationResult{
				Success: false,
				Error:   err.Error(),
			},
		}, nil
	}

	return &pb.RetryNotificationResponse{
		Result: &pb.NotificationResult{
			NotificationId: result.NotificationID,
			Success:        result.Success,
			Message:        result.Message,
			SentAt:         timestamppb.New(result.SentAt),
		},
	}, nil
}

// GetStats returns notification statistics
func (h *NotifierHandler) GetStats(ctx context.Context, req *pb.GetStatsRequest) (*pb.GetStatsResponse, error) {
	stats, err := h.service.GetStats(ctx)
	if err != nil {
		return nil, err
	}

	return &pb.GetStatsResponse{
		TotalSent:    stats.TotalSent,
		TotalFailed:  stats.TotalFailed,
		TotalPending: stats.TotalPending,
		TotalQueued:  stats.TotalQueued,
		ByType:       stats.ByType,
		ByStatus:     stats.ByStatus,
	}, nil
}

// GetNotifiers returns information about available notifiers
func (h *NotifierHandler) GetNotifiers(ctx context.Context, req *pb.GetNotifiersRequest) (*pb.GetNotifiersResponse, error) {
	h.logger.Infof("gRPC: Received request for available notifiers")

	notifiers, err := h.service.GetNotifiers(ctx)
	if err != nil {
		h.logger.Errorf("gRPC: Failed to get notifiers - error=%v", err)
		return nil, err
	}

	// Convert domain notifiers to proto notifiers
	protoNotifiers := make([]*pb.NotifierInfo, 0, len(notifiers.Notifiers))
	for _, notifier := range notifiers.Notifiers {
		protoNotifiers = append(protoNotifiers, &pb.NotifierInfo{
			Type:           convertDomainTypeToProto(notifier.Type),
			Accounts:       notifier.Accounts,
			DefaultAccount: notifier.DefaultAccount,
		})
	}

	return &pb.GetNotifiersResponse{
		Notifiers: protoNotifiers,
	}, nil
}

// Helper functions to convert between proto and domain types

// convertStringMapToInterface converts proto's map[string]string to domain's map[string]interface{}
func convertStringMapToInterface(m map[string]string) map[string]interface{} {
	if m == nil {
		return nil
	}
	result := make(map[string]interface{}, len(m))
	for k, v := range m {
		result[k] = v
	}
	return result
}

// convertInterfaceMapToString converts domain's map[string]interface{} to proto's map[string]string
func convertInterfaceMapToString(m map[string]interface{}) map[string]string {
	if m == nil {
		return nil
	}
	result := make(map[string]string, len(m))
	for k, v := range m {
		if v != nil {
			result[k] = fmt.Sprint(v)
		}
	}
	return result
}

func convertProtoTypeToDomain(protoType pb.NotificationType) domain.NotificationType {
	switch protoType {
	case pb.NotificationType_NOTIFICATION_TYPE_EMAIL:
		return domain.TypeEmail
	case pb.NotificationType_NOTIFICATION_TYPE_SLACK:
		return domain.TypeSlack
	case pb.NotificationType_NOTIFICATION_TYPE_NTFY:
		return domain.TypeNtfy
	case pb.NotificationType_NOTIFICATION_TYPE_STDOUT:
		return domain.TypeStdout
	default:
		return domain.TypeStdout
	}
}

func convertDomainTypeToProto(domainType domain.NotificationType) pb.NotificationType {
	switch domainType {
	case domain.TypeEmail:
		return pb.NotificationType_NOTIFICATION_TYPE_EMAIL
	case domain.TypeSlack:
		return pb.NotificationType_NOTIFICATION_TYPE_SLACK
	case domain.TypeNtfy:
		return pb.NotificationType_NOTIFICATION_TYPE_NTFY
	case domain.TypeStdout:
		return pb.NotificationType_NOTIFICATION_TYPE_STDOUT
	default:
		return pb.NotificationType_NOTIFICATION_TYPE_UNSPECIFIED
	}
}

func convertProtoContentTypeToDomain(protoType pb.ContentType) domain.ContentType {
	switch protoType {
	case pb.ContentType_CONTENT_TYPE_HTML:
		return domain.ContentTypeHTML
	case pb.ContentType_CONTENT_TYPE_TEXT:
		return domain.ContentTypeText
	default:
		return domain.ContentTypeText // Default to text
	}
}

func convertDomainContentTypeToProto(domainType domain.ContentType) pb.ContentType {
	switch domainType {
	case domain.ContentTypeHTML:
		return pb.ContentType_CONTENT_TYPE_HTML
	case domain.ContentTypeText:
		return pb.ContentType_CONTENT_TYPE_TEXT
	default:
		return pb.ContentType_CONTENT_TYPE_TEXT
	}
}

func convertDomainToProtoType(domainType domain.NotificationType) pb.NotificationType {
	switch domainType {
	case domain.TypeEmail:
		return pb.NotificationType_NOTIFICATION_TYPE_EMAIL
	case domain.TypeSlack:
		return pb.NotificationType_NOTIFICATION_TYPE_SLACK
	case domain.TypeNtfy:
		return pb.NotificationType_NOTIFICATION_TYPE_NTFY
	case domain.TypeStdout:
		return pb.NotificationType_NOTIFICATION_TYPE_STDOUT
	default:
		return pb.NotificationType_NOTIFICATION_TYPE_UNSPECIFIED
	}
}

func convertDomainToProtoStatus(status domain.NotificationStatus) pb.NotificationStatus {
	switch status {
	case domain.StatusPending:
		return pb.NotificationStatus_NOTIFICATION_STATUS_PENDING
	case domain.StatusQueued:
		return pb.NotificationStatus_NOTIFICATION_STATUS_QUEUED
	case domain.StatusProcessing:
		return pb.NotificationStatus_NOTIFICATION_STATUS_PROCESSING
	case domain.StatusSent:
		return pb.NotificationStatus_NOTIFICATION_STATUS_SENT
	case domain.StatusFailed:
		return pb.NotificationStatus_NOTIFICATION_STATUS_FAILED
	case domain.StatusRetrying:
		return pb.NotificationStatus_NOTIFICATION_STATUS_RETRYING
	default:
		return pb.NotificationStatus_NOTIFICATION_STATUS_UNSPECIFIED
	}
}

func convertDomainToProtoNotification(notif *domain.Notification) *pb.Notification {
	protoNotif := &pb.Notification{
		Id:         notif.ID,
		Type:       convertDomainToProtoType(notif.Type),
		Account:    notif.Account,
		Priority:   pb.Priority(notif.Priority),
		Status:     convertDomainToProtoStatus(notif.Status),
		Subject:    notif.Subject,
		Body:       notif.Body,
		Recipients: notif.Recipients,
		Metadata:   convertInterfaceMapToString(notif.Metadata),
		CreatedAt:  timestamppb.New(notif.CreatedAt),
		RetryCount: int32(notif.RetryCount),
		MaxRetries: int32(notif.MaxRetries),
		LastError:  notif.LastError,
	}

	// Handle optional timestamp fields
	if notif.ScheduledFor != nil {
		protoNotif.ScheduledFor = timestamppb.New(*notif.ScheduledFor)
	}
	if notif.SentAt != nil {
		protoNotif.SentAt = timestamppb.New(*notif.SentAt)
	}

	return protoNotif
}

func convertProtoFilterToDomain(filter *pb.NotificationFilter) *domain.NotificationFilter {
	if filter == nil {
		return &domain.NotificationFilter{}
	}

	// Convert proto types to domain types
	var types []domain.NotificationType
	for _, protoType := range filter.Types {
		types = append(types, convertProtoTypeToDomain(protoType))
	}

	// Convert proto statuses to domain statuses
	var statuses []domain.NotificationStatus
	for _, protoStatus := range filter.Statuses {
		statuses = append(statuses, convertProtoStatusToDomain(protoStatus))
	}

	domainFilter := &domain.NotificationFilter{
		IDs:        filter.Ids,
		Types:      types,
		Statuses:   statuses,
		Recipients: filter.Recipients,
		Limit:      int(filter.Limit),
		Offset:     int(filter.Offset),
	}

	if filter.CreatedAfter != nil {
		createdAfter := filter.CreatedAfter.AsTime()
		domainFilter.CreatedAfter = &createdAfter
	}

	if filter.CreatedBefore != nil {
		createdBefore := filter.CreatedBefore.AsTime()
		domainFilter.CreatedBefore = &createdBefore
	}

	return domainFilter
}

func convertProtoStatusToDomain(protoStatus pb.NotificationStatus) domain.NotificationStatus {
	switch protoStatus {
	case pb.NotificationStatus_NOTIFICATION_STATUS_PENDING:
		return domain.StatusPending
	case pb.NotificationStatus_NOTIFICATION_STATUS_QUEUED:
		return domain.StatusQueued
	case pb.NotificationStatus_NOTIFICATION_STATUS_PROCESSING:
		return domain.StatusProcessing
	case pb.NotificationStatus_NOTIFICATION_STATUS_SENT:
		return domain.StatusSent
	case pb.NotificationStatus_NOTIFICATION_STATUS_FAILED:
		return domain.StatusFailed
	case pb.NotificationStatus_NOTIFICATION_STATUS_RETRYING:
		return domain.StatusRetrying
	default:
		return domain.StatusPending
	}
}
