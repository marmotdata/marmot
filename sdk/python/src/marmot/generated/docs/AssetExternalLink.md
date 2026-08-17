# AssetExternalLink


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**icon** | **str** |  | [optional] 
**name** | **str** |  | [optional] 
**url** | **str** |  | [optional] 

## Example

```python
from marmot.generated.models.asset_external_link import AssetExternalLink

# TODO update the JSON string below
json = "{}"
# create an instance of AssetExternalLink from a JSON string
asset_external_link_instance = AssetExternalLink.from_json(json)
# print the JSON string representation of the object
print(AssetExternalLink.to_json())

# convert the object into a dict
asset_external_link_dict = asset_external_link_instance.to_dict()
# create an instance of AssetExternalLink from a dict
asset_external_link_from_dict = AssetExternalLink.from_dict(asset_external_link_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


