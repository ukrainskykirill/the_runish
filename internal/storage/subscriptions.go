package storage

import "context"

func (s *Store) RefreshSubscriptionEntitlement(ctx context.Context, subID int64) error {
	return refreshSubEntitlementTx(ctx, s.db, subID, false)
}
