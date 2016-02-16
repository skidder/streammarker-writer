Feature: Sensor Readings API
  As a user of Fabric
  I want to be able to record sensor readings

  Background:
    Given I have a clean database

  @happy
  Scenario: Record a single message from the queue
    Given I have the account "account1" with active Relay "53644F1C-2480-4F9B-9CBA-26D66139D221"
    When I have the queued Sensor data "valid_single_sensor_reading" waiting on the SQS queue
    And sleep 2
    Then the queue should have 0 messages visible
    And the Sensor Readings table should have a record for account "account1" and sensor "556AC569-6E7D-44A9-A64C-D900927010FE"
    And the reading has a "Celsius" temperature measurement of 28.3
    And the reading has no location data
    And the Hourly Sensor Readings table should have a record for account "account1" and sensor "556AC569-6E7D-44A9-A64C-D900927010FE" with a min of "28.3" and max of "28.3"

  @happy
  Scenario: Process two messages where the later is ignored due to sample frequency
    Given I have the account "account1" with active Relay "53644F1C-2480-4F9B-9CBA-26D66139D221"
    And I have the account "account1" with active Sensor "ACA4C42F-18FD-4038-AACD-DE575E261E7A"
    When I have the queued Sensor data "sample_frequency_msg1" waiting on the SQS queue
    And I have the queued Sensor data "sample_frequency_msg2" waiting on the SQS queue
    And sleep 2
    Then the queue should have 0 messages visible
    And the Sensor Readings table should have a record for account "account1" and sensor "ACA4C42F-18FD-4038-AACD-DE575E261E7A"
    And the reading has a "Celsius" temperature measurement of 28.3
    And the reading has a location of 100.2, 150.1

  @happy
  Scenario: Record two messages from the queue
    Given I have the account "account1" with active Relay "53644F1C-2480-4F9B-9CBA-26D66139D221"
    When I have the queued Sensor data "valid_single_sensor_reading" waiting on the SQS queue
    And I have the queued Sensor data "valid_max_single_sensor_reading" waiting on the SQS queue
    And sleep 2
    Then the queue should have 0 messages visible
    And the Sensor Readings table should have a record for account "account1" and sensor "556AC569-6E7D-44A9-A64C-D900927010FE"
    And the Hourly Sensor Readings table should have a record for account "account1" and sensor "556AC569-6E7D-44A9-A64C-D900927010FE" with a min of "28.3" and max of "31.2"

  @happy
  Scenario: Record three messages from the queue
    Given I have the account "account1" with active Relay "53644F1C-2480-4F9B-9CBA-26D66139D221"
    When I have the queued Sensor data "valid_single_sensor_reading" waiting on the SQS queue
    And I have the queued Sensor data "valid_max_single_sensor_reading" waiting on the SQS queue
    And I have the queued Sensor data "valid_middle_single_sensor_reading" waiting on the SQS queue
    And sleep 2
    Then the queue should have 0 messages visible
    And the Sensor Readings table should have a record for account "account1" and sensor "556AC569-6E7D-44A9-A64C-D900927010FE"
    And the Hourly Sensor Readings table should have a record for account "account1" and sensor "556AC569-6E7D-44A9-A64C-D900927010FE" with a min of "28.3" and max of "31.2"

  @sad
  Scenario: Handle a single message with conflicting account id
    Given I have the account "account1" with active Relay "53644F1C-2480-4F9B-9CBA-26D66139D221"
    And I have the account "account2" with active Sensor "ACA4C42F-18FD-4038-AACD-DE575E261E7A"
    When I have the queued Sensor data "valid_single_sensor_reading_for_account1" waiting on the SQS queue
    And sleep 2
    Then the queue should have 0 messages visible
    And the Sensor Readings table should be nonexistent

  @sad
  Scenario: Single sensor message with no measurements
    Given I have the account "account1" with active Relay "53644F1C-2480-4F9B-9CBA-26D66139D222"
    When I have the queued Sensor data "empty_single_sensor_reading" waiting on the SQS queue
    And sleep 2
    Then the queue should have 0 messages visible
    And the Sensor Readings table should be nonexistent

  @sad
  Scenario: Process message for unrecognized reporting device
    Given I have the queued Sensor data "bad_reporting_device_single_sensor_reading" waiting on the SQS queue
    When sleep 2
    Then the queue should have 0 messages visible
    And the Sensor Readings table should be nonexistent

  @sad
  Scenario: Process message for inactive reporting device
    Given I have the account "account3" with inactive Relay "53644F1C-2480-4F9B-9CBA-26D66139D221"
    When I have the queued Sensor data "valid_single_sensor_reading" waiting on the SQS queue
    And sleep 2
    Then the queue should have 0 messages visible
    And the Sensor Readings table should be nonexistent
