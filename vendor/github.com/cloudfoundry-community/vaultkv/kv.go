package vaultkv

//Get retrieves the secret at the given path and unmarshals it into the given
//output object. If the object is nil, an unmarshal will not be attempted (this
//can be used to check for existence). If the object could not be unmarshalled
//into, the resultant error is returned. Example path would be /secret/foo, if
//Key/Value backend were mounted at "/secret". The Vault must be unsealed and
// initialized for this endpoint to work. No assumptions are made about the
// mounting point of your Key/Value backend.
func (v *Client) Get(path string, output interface{}) error {
	var unmarshalInto interface{}
	if output != nil {
		unmarshalInto = &vaultResponse{Data: &output}
	}

	err := v.doRequest("GET", path, nil, unmarshalInto)
	if err != nil {
		return err
	}

	return err
}

//List returns the list of paths nested directly under the given path. If this
//is not a "directory" for any paths, then ErrNotFound is returned. In the list
//of paths returned on success, if a path ends with a slash, then it is also a
//"directory". The Vault must be unsealed and initialized for this endpoint to
//work. No assumptions are made about the mounting point of your Key/Value
//backend.
func (v *Client) List(path string) ([]string, error) {
	ret := []string{}

	err := v.doRequest("LIST", path, nil, &vaultResponse{
		Data: &struct {
			Keys *[]string `json:"keys"`
		}{
			Keys: &ret,
		},
	})
	if err != nil {
		return nil, err
	}

	return ret, err
}

//Set puts the values in the given map at the given path. The keys in the map
//become the keys at the path, and the values in the map become the values of
//those keys. The Vault must be unsealed and initialized for this endpoint to
//work. No assumptions are made about the mounting point of your Key/Value
//backend.
func (v *Client) Set(path string, values map[string]string) error {
	err := v.doRequest("PUT", path, &values, nil)
	if err != nil {
		return err
	}

	return nil
}

//Delete attempts to delete the value at the specified path. No error is
//returned if there is already no value at the given path.
func (v *Client) Delete(path string) error {
	err := v.doRequest("DELETE", path, nil, nil)
	if err != nil {
		return err
	}

	return nil
}
