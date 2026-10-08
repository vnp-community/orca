package domain

// ChangeTypeAllowed encodes the upgrade paths of README v6 section 3.4: only widening
// moves keep existing work meaningful; everything else is a new Request.
func ChangeTypeAllowed(from, to RequestType) error {
	if from == to {
		return ErrRequestTypeUnchanged(to)
	}
	switch from {
	case RequestTypeSpike, RequestTypeQuestion, RequestTypeHotfix:
		return ErrRequestTypeChangeUseChild(from, to)
	case RequestTypeBug, RequestTypeTask, RequestTypeRefactor, RequestTypePerformance:
		if to == RequestTypeChangeRequest {
			return nil
		}
	case RequestTypeDocs:
		if to == RequestTypeTask || to == RequestTypeChangeRequest {
			return nil
		}
	case RequestTypeSecurity:
		if to == RequestTypeHotfix {
			return nil
		}
	}
	return ErrRequestTypeChangeNotAllowed(from, to)
}
