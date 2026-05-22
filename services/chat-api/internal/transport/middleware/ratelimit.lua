local current = redis.call('INCR', KEYS[1])
if current == 1 then
    redis.call('EXPIRE', KEYS[1], 2)
end
return current <= tonumber(ARGV[1])
