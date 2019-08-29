package server

import "encoding/json"

// minifyJSON - Unmarshall JSON and marshall it to remove comments and whitespaces
func minifyJSON(input json.RawMessage) (json.RawMessage, error) {

	// Unmarshall raw JSON to Object
	inputObject := &json.RawMessage{}
	err := json.Unmarshal(input, inputObject)
	if err != nil {
		return json.RawMessage{}, err
	}

	// Marshall object to minified raw JSON
	output, err := json.Marshal(inputObject)
	if err != nil {
		return json.RawMessage{}, nil
	}

	return output, nil
}
