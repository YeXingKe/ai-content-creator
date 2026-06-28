-- 将初始账号密码哈希更新为盐值 mason（明文密码仍为 12345678）
-- MD5('12345678' + 'mason') = cf0bada5c2fcee97fc65b9f2534ac461

USE ai_content_creator;

UPDATE user
SET userPassword = 'cf0bada5c2fcee97fc65b9f2534ac461'
WHERE userAccount IN ('admin', 'user', 'test');
