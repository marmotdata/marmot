# AggregatedMetric


## Properties

Name | Type | Description | Notes
------------ | ------------- | ------------- | -------------
**aggregation_type** | **str** |  | [optional] 
**bucket_end** | **str** |  | [optional] 
**bucket_size** | **int** |  | [optional] 
**bucket_start** | **str** |  | [optional] 
**labels** | **Dict[str, str]** |  | [optional] 
**name** | **str** |  | [optional] 
**value** | **float** |  | [optional] 

## Example

```python
from marmot.generated.models.aggregated_metric import AggregatedMetric

# TODO update the JSON string below
json = "{}"
# create an instance of AggregatedMetric from a JSON string
aggregated_metric_instance = AggregatedMetric.from_json(json)
# print the JSON string representation of the object
print(AggregatedMetric.to_json())

# convert the object into a dict
aggregated_metric_dict = aggregated_metric_instance.to_dict()
# create an instance of AggregatedMetric from a dict
aggregated_metric_from_dict = AggregatedMetric.from_dict(aggregated_metric_dict)
```
[[Back to Model list]](../README.md#documentation-for-models) [[Back to API list]](../README.md#documentation-for-api-endpoints) [[Back to README]](../README.md)


