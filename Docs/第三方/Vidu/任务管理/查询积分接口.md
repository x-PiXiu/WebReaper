# 查询积分接口

`GET https://api.vidu.cn/ent/v2/credits`

## 请求头

| 字段 | 值 | 描述 |
| --- | --- | --- |
| Content-Type | application/json | 数据交换格式 |
| Authorization | Token `{your api key}` | 将 `{your api key}` 替换为您的 token |

## 请求体

| 参数名称 | 类型 | 必填 | 参数描述 |
| --- | --- | --- | --- |
| show_detail | Boolean | 是 | 是否展示全部生效中的资源包详情<br>默认：false，枚举值：true、false |

```
curl -X GET -H "Authorization: Token {your_api_key}" https://api.vidu.cn/ent/v2/credits
```

## 响应体

| 字段 | 子字段 | 类型 | 描述 |
| --- | --- | --- | --- |
| remaining_credits |  | Array | 剩余可用积分总数 |
|  | type | String | 资源包类型<br>可选值：<br>`test` 测试资源包、<br>`metered` 积分资源包、<br>`concurrent` 并发资源包 |
|  | credit_remain | Int | 该类型资源包剩余的总积分 |
|  | concurrency_limit | Int | 可用并发数上限 |
|  | current_concurrency | Int | 当前已使用并发数 |
|  | queue_count | Int | 当前排队任务数 |
| packages |  | Array | 资源包详情 |
|  | id | String | 资源包 id |
|  | name | String | 资源包名称 |
|  | type | String | 资源包类型<br>可选值：<br>`test` 测试资源包、<br>`metered` 积分资源包、<br>`concurrent` 并发资源包 |
|  | concurrency | Int | 并发数 |
|  | credit_amount | Int | 积分总量 |
|  | credit_remain | Int | 积分余量 |
|  | created_at | String | 订单创建时间 |
|  | purchase_at | String | 购买时间 |
|  | valid_from | String | 生效时间 |
|  | valid_to | String | 失效时间 |

```
{
    "remains": [
        {
            "type": "your_package_type",
            "credit_remain": your_credit_remain,
            "concurrency_limit": your_concurrency_limit,
            "current_concurrency": your_current_concurrency,
            "queue_count": 20
        }
    ],
      "packages": [
        {
            "id": "your_packages_id",
            "name": "your_packages_name",
            "type": "your_package_type",
            "concurrency": your_package_concurrency_limit,
            "credit_amount": your_package_credit_total_amount,
            "credit_remain": your_package_credit_remain_amount,
            "created_at": "your_package_created_time",
            "purchase_at": "your_package_purchase_time",
            "valid_from": "your_package_valid_from",
            "valid_to": "your_package_valid_to"
        }
    ]
}
```