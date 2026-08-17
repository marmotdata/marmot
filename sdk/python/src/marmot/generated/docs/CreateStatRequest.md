# CreateStatRequest


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**asset_mrn** | **str** |  | 
**metric_name** | **str** |  | 
**value** | **float** |  | 

## Example

```python
from marmot.generated.models.create_stat_request import CreateStatRequest

# TODO update the JSON string below
json = "{}"
# create an instance of CreateStatRequest from a JSON string
create_stat_request_instance = CreateStatRequest.from_json(json)
# print the JSON string representation of the object
print(CreateStatRequest.to_json())

# convert the object into a dict
create_stat_request_dict = create_stat_request_instance.to_dict()
# create an instance of CreateStatRequest from a dict
create_stat_request_from_dict = CreateStatRequest.from_dict(create_stat_request_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


