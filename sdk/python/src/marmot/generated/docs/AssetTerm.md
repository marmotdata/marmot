# AssetTerm


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**created_at** | **str** |  | [optional] 
**created_by** | **str** |  | [optional] 
**created_by_username** | **str** |  | [optional] 
**definition** | **str** |  | [optional] 
**source** | **str** | \&quot;user\&quot; or \&quot;plugin:name\&quot; | [optional] 
**term_id** | **str** |  | [optional] 
**term_name** | **str** |  | [optional] 

## Example

```python
from marmot.generated.models.asset_term import AssetTerm

# TODO update the JSON string below
json = "{}"
# create an instance of AssetTerm from a JSON string
asset_term_instance = AssetTerm.from_json(json)
# print the JSON string representation of the object
print(AssetTerm.to_json())

# convert the object into a dict
asset_term_dict = asset_term_instance.to_dict()
# create an instance of AssetTerm from a dict
asset_term_from_dict = AssetTerm.from_dict(asset_term_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


