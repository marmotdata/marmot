# TokenExchangeResponse


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**access_token** | **str** |  | [optional] 
**expires_in** | **int** |  | [optional] 
**issued_token_type** | **str** |  | [optional] 
**token_type** | **str** |  | [optional] 

## Example

```python
from marmot.generated.models.token_exchange_response import TokenExchangeResponse

# TODO update the JSON string below
json = "{}"
# create an instance of TokenExchangeResponse from a JSON string
token_exchange_response_instance = TokenExchangeResponse.from_json(json)
# print the JSON string representation of the object
print(TokenExchangeResponse.to_json())

# convert the object into a dict
token_exchange_response_dict = token_exchange_response_instance.to_dict()
# create an instance of TokenExchangeResponse from a dict
token_exchange_response_from_dict = TokenExchangeResponse.from_dict(token_exchange_response_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


