package main

func Main(args map[string]interface{}) map[string]interface{} {
	email, ok := args["email"].(string)
	msg := make(map[string]interface{})
	if !ok {
		msg["statusCode"] = 400
		msg["body"] = "Missing email"
	}

	msg["body"] = "Subscribing " + email + "!"
	return msg
}
