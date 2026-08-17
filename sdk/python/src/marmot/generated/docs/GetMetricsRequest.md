# GetMetricsRequest


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**aggregation** | **str** |  | [optional] 
**bucket_size** | **str** |  | [optional] 
**end** | **str** |  | [optional] 
**labels** | **Dict[str, str]** |  | [optional] 
**metric_names** | **List[str]** |  | [optional] 
**start** | **str** |  | [optional] 

## Example

```python
from marmot.generated.models.get_metrics_request import GetMetricsRequest

# TODO update the JSON string below
json = "{}"
# create an instance of GetMetricsRequest from a JSON string
get_metrics_request_instance = GetMetricsRequest.from_json(json)
# print the JSON string representation of the object
print(GetMetricsRequest.to_json())

# convert the object into a dict
get_metrics_request_dict = get_metrics_request_instance.to_dict()
# create an instance of GetMetricsRequest from a dict
get_metrics_request_from_dict = GetMetricsRequest.from_dict(get_metrics_request_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


