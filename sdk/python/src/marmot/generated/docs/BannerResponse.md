# BannerResponse


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**dismissible** | **bool** |  | [optional] 
**enabled** | **bool** |  | [optional] 
**id** | **str** |  | [optional] 
**message** | **str** |  | [optional] 
**variant** | **str** |  | [optional] 

## Example

```python
from marmot.generated.models.banner_response import BannerResponse

# TODO update the JSON string below
json = "{}"
# create an instance of BannerResponse from a JSON string
banner_response_instance = BannerResponse.from_json(json)
# print the JSON string representation of the object
print(BannerResponse.to_json())

# convert the object into a dict
banner_response_dict = banner_response_instance.to_dict()
# create an instance of BannerResponse from a dict
banner_response_from_dict = BannerResponse.from_dict(banner_response_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


