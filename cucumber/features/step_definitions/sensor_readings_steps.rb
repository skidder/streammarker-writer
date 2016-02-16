Given(/^I have the account "(.*)" with (.*) Relay "(.*)"$/) do |account_id, state, relay_id|
  put_relay_record(account_id, relay_id, state)
end

Given(/^I have the account "(.*?)" with active Sensor "(.*?)"$/) do |account_id, sensor_id|
  put_sensor_record(account_id, sensor_id)
end

Given(/^I have the queued Sensor data "(.*)" waiting on the SQS queue$/) do |name|
  @queued_message_body = get_request(name)
  send_message_to_queue(ENV['STREAMMARKER_SQS_QUEUE_URL'], @queued_message_body)
end

Then(/^the queue should have (.*) messages visible$/) do |expected_queue_length|
  actual = number_of_messages_visible_in_queue(ENV['STREAMMARKER_SQS_QUEUE_URL'])
  actual.should eq(expected_queue_length)
end

Then(/^the Sensor Readings table should have a record for account "(.*)" and sensor "(.*)"$/) do |account_id, sensor_id|
  @rec = get_sensor_reading_dynamo_record(account_id, sensor_id)
  @rec.should_not be_nil
end

Then(/^the reading has a "(.*?)" temperature measurement of (.*?)$/) do |unit, value|
  @rec["measurements"].should_not be_nil
  JSON.parse(@rec["measurements"])[0]["unit"].should eq(unit)
  JSON.parse(@rec["measurements"])[0]["value"].should eq(value.to_f)
end

Then(/^the reading has a location of (.*?),(.*?)$/) do |lat, lon|
  @rec["measurements"].should_not be_nil
  @rec["latitude"].should eq(lat.to_f)
  @rec["longitude"].should eq(lon.to_f)
end

Then(/^the reading has no location data$/) do
  @rec["measurements"].should_not be_nil
  @rec["latitude"].should be_nil
  @rec["longitude"].should be_nil
end

Then(/^the Sensor Readings table should be nonexistent$/) do
  sensor_readings_table_exists?.should be(false)
end

Then(/^the Sensor Readings table should be empty for account "(.*)" and sensor "(.*)"$/) do |account_id, sensor_id|
  rec = get_sensor_reading_dynamo_record(account_id, sensor_id)
  rec.should be_nil
end

Then(/^the Hourly Sensor Readings table should have a record for account "(.*?)" and sensor "(.*?)" with a min of "(.*?)" and max of "(.*?)"$/) do |account_id, sensor_id, minReading, maxReading|
  rec = get_sensor_hourly_readings_dynamo_record(account_id, sensor_id)
  rec.should_not be_nil
  rec["measurements"].should_not be_nil
  JSON.parse(rec["measurements"])[0]["min"]["value"].to_f.should eq(minReading.to_f)
  JSON.parse(rec["measurements"])[0]["max"]["value"].to_f.should eq(maxReading.to_f)
end

Then(/^the Sensor Readings table should have "(.*?)" records for account "(.*?)" and sensor "(.*?)"$/) do |item_count, account_id, sensor_id|
  get_sensor_reading_count(account_id, sensor_id).should eq(item_count.to_i)
end