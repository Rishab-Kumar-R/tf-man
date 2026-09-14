local reservationKey = KEYS[1]
local stockKey = KEYS[2]
local qty = tonumber(ARGV[1])
local itemID = ARGV[2]

if redis.call("EXISTS", reservationKey) == 1 then
      return 1
end

local stock = tonumber(redis.call("GET", stockKey) or "0")
if stock < qty then
      return 0
end

local remaining = redis.call("DECRBY", stockKey, qty)
redis.call("SET", reservationKey, qty, "EX", 86400)
redis.call("PUBLISH", "stock-updates", cjson.encode({item_id = itemID, remaining = remaining}))
return 1
