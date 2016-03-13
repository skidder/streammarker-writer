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
  @recs = get_latest_sensor_reading_influxdb_record(account_id, sensor_id)
  @recs.should_not be_nil
  @recs.should_not be_empty
  @rec = @recs[0]
end

Then(/^the reading has a "(.*?)" temperature measurement of (.*?)$/) do |unit, value|
  @rec["values"].should_not be_nil
  @rec["values"].should_not be_empty
  @rec["values"][0]["unit"].should eq(unit)
  @rec["values"][0]["value"].should eq(value.to_f)
end

Then(/^the reading has a location of (.*?),(.*?)$/) do |lat, lon|
  @rec["values"].should_not be_nil
  @rec["values"].should_not be_empty
  @rec["values"][0]["latitude"].should eq(lat.to_f)
  @rec["values"][0]["longitude"].should eq(lon.to_f)
end

Then(/^the reading has no location data$/) do
  @rec["values"].should_not be_nil
  @rec["values"][0]["latitude"].should be_nil
  @rec["values"][0]["longitude"].should be_nil
end

Then(/^the Sensor Readings table should be nonexistent$/) do
  sensor_readings_table_exists?.should be(false)
end

Then(/^the Sensor Readings table should be empty for account "(.*)" and sensor "(.*)"$/) do |account_id, sensor_id|
  get_latest_sensor_reading_influxdb_record(account_id, sensor_id).should be_nil  
end

Then(/^the Sensor Readings table should have "(.*?)" records for account "(.*?)" and sensor "(.*?)"$/) do |item_count, account_id, sensor_id|
  get_sensor_reading_count(account_id, sensor_id).should eq(item_count.to_i)
end