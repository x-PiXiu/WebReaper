-- 035_brand_website_url.sql 品牌官网地址（online 品牌 NAP）
--
-- 背景：online 品牌（SaaS/工具/网络公司）无线下门店——其"NAP"（Name, Address, Phone）
--   对应的是 Name + WebsiteURL + ProductDescription。内容生成注入"了解更多：https://..."，
--   收录提交用、公开站链接用。local 品牌也可填（官网 + 门店并存）。

ALTER TABLE geo_brands ADD COLUMN website_url VARCHAR(256) DEFAULT '';
