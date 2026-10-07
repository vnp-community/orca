package usecase

import "context"

func CallExternalTool(ctx context.Context, toolName string, args map[string]interface{}) (interface{}, error) {
	return nil, nil
}

func ReadResource(ctx context.Context, uri string) (string, error) {
	return "", nil
}
