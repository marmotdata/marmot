# AssetsByOwnerResponse


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**assets** | **Dict[str, int]** |  | [optional] 

## Example

```python
from marmot.generated.models.assets_by_owner_response import AssetsByOwnerResponse

# TODO update the JSON string below
json = "{}"
# create an instance of AssetsByOwnerResponse from a JSON string
assets_by_owner_response_instance = AssetsByOwnerResponse.from_json(json)
# print the JSON string representation of the object
print(AssetsByOwnerResponse.to_json())

# convert the object into a dict
assets_by_owner_response_dict = assets_by_owner_response_instance.to_dict()
# create an instance of AssetsByOwnerResponse from a dict
assets_by_owner_response_from_dict = AssetsByOwnerResponse.from_dict(assets_by_owner_response_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


