Given(/^I have a clean database$/) do
  begin
    teardown_tables
  rescue Exception => e
    # ignore    
  end

  setup_tables
end

When(/^sleep [-+]?([0-9]*\.[0-9]+|[0-9]+) seconds$/) do |seconds|
  sleep seconds.to_i
end
