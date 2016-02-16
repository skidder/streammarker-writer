def get_request(name)
  get_json_from_fixture_file_as_hash('requests.json', name).to_json
end

def get_response(name)
  get_json_from_fixture_file_as_hash('responses.json', name).to_json
end

def get_json_from_fixture_file_as_hash(file, name)
  request = get_fixture_file_as_string(file)
  json = JSON.parse(request)[name]
  raise "Unable to find key '#{name}' in '#{file}'" if json.nil?
  json
end

def get_fixture_file_as_string(filename)
  File.read(File.join(CUCUMBER_BASE, 'fixtures', filename))
end

def get_sensor_hourly_readings_dynamo_record(account_id, sensor_id)
  ddb = get_dynamo_client
  table_name = "hourly_sensor_readings_#{Time.at(1433031540).strftime('%Y-%m')}"
  resp = ddb.query(table_name: table_name,
    limit: 1,
    key_conditions: {
      "id" => {
        attribute_value_list: [
          "#{account_id}:#{sensor_id}",
        ],
        comparison_operator: "EQ",
      }
    })[:items].first
end

def get_sensor_reading_dynamo_record(account_id, sensor_id)
  ddb = get_dynamo_client
  table_name = "sensor_readings_#{Time.at(1433031540).strftime('%Y-%m')}"
  resp = ddb.query(table_name: table_name,
    limit: 1,
    key_conditions: {
      "id" => {
        attribute_value_list: [
          "#{account_id}:#{sensor_id}",
        ],
        comparison_operator: "EQ",
      }
    })[:items].first
end

def get_sensor_reading_count(account_id, sensor_id)
  ddb = get_dynamo_client
  table_name = "sensor_readings_#{Time.at(1433031540).strftime('%Y-%m')}"
  resp = ddb.query(table_name: table_name,
    limit: 1000,
    key_conditions: {
      "id" => {
        attribute_value_list: [
          "#{account_id}:#{sensor_id}",
        ],
        comparison_operator: "EQ",
      }
    })[:items].count
end

def put_relay_record(account_id, relay_id, state="active")
  ddb = get_dynamo_client
  ddb.put_item(table_name: "relays",
               item: {
                 "account_id" => account_id,
                 "name" => "Relay",
                 "id" => relay_id,
                 "state" => state,
                }
              )
end

def put_sensor_record(account_id, sensor_id, state="active")
  ddb = get_dynamo_client
  ddb.put_item(table_name: "sensors",
               item: {
                 "account_id" => account_id,
                 "name" => "Sensor",
                 "id" => sensor_id,
                 "state" => state,
                 "location_enabled" => true,
                 "latitude" => 100.2,
                 "longitude" => 150.1,
                 "sample_frequency" => 1
                }
              )
end

def send_message_to_queue(queue, body)
  sqs = AWS::SQS.new(:access_key_id   => 'x',
                   :secret_access_key => 'y',
                   :use_ssl           => false,
                   :sqs_endpoint      => FAKESQS_HOST,
                   :sqs_port          => FAKESQS_PORT.to_i
                   )
  resp = sqs.client.send_message(queue_url: queue, message_body: body)
end

def number_of_messages_visible_in_queue(queue)
  sqs = AWS::SQS.new(:access_key_id   => 'x',
                   :secret_access_key => 'y',
                   :use_ssl           => false,
                   :sqs_endpoint      => FAKESQS_HOST,
                   :sqs_port          => FAKESQS_PORT.to_i
                   )

  resp = sqs.client.get_queue_attributes(
    queue_url: queue,
    attribute_names: ["ApproximateNumberOfMessages"],
  )
  resp[:attributes]["ApproximateNumberOfMessages"]
end

def sensor_readings_table_exists?
  ddb = get_dynamo_client
  table_name = "sensor_readings_#{Time.at(1433031540).strftime('%Y-%m')}"
  begin
    resp = ddb.describe_table({
      table_name: table_name,
    })
    return (resp.table ? true : false)
  rescue Aws::DynamoDB::Errors::ResourceNotFoundException
    return false
  end
end

def setup_tables
  ddb = get_dynamo_client
  ddb.create_table(
      attribute_definitions: [{
                                  attribute_name: "id",
                                  attribute_type: "S",
                              }],
      table_name: "accounts",
      key_schema: [{
                       attribute_name: "id",
                       key_type: "HASH",
                   }],
      provisioned_throughput: {
          read_capacity_units: 1,
          write_capacity_units: 1,
      })

  ddb.create_table(
      attribute_definitions: [{
                                  attribute_name: "id",
                                  attribute_type: "S",
                              }],
      table_name: "relays",
      key_schema: [{
                       attribute_name: "id",
                       key_type: "HASH",
                   }],
      provisioned_throughput: {
          read_capacity_units: 1,
          write_capacity_units: 1,
      })

  ddb.create_table(
      attribute_definitions: [{
                                  attribute_name: "id",
                                  attribute_type: "S",
                              }],
      table_name: "sensors",
      key_schema: [{
                       attribute_name: "id",
                       key_type: "HASH",
                   }],
      provisioned_throughput: {
          read_capacity_units: 1,
          write_capacity_units: 1,
      })
end  

def silently_delete_table(table_name)
  ddb = get_dynamo_client
  begin
    ddb.delete_table(table_name: table_name)
  rescue Aws::DynamoDB::Errors::ResourceNotFoundException
  end
end

def teardown_tables
  silently_delete_table("relays")
  silently_delete_table("sensors")
  silently_delete_table("accounts")
  silently_delete_table("sensor_readings_#{Time.at(1433031540).strftime('%Y-%m')}")
  silently_delete_table("hourly_sensor_readings_#{Time.at(1433031540).strftime('%Y')}")
end

def get_dynamo_client
  Aws::DynamoDB::Client.new(
      access_key_id: 'x',
      secret_access_key: 'y',
      endpoint: ENV['STREAMMARKER_DYNAMO_ENDPOINT']
  )  
end