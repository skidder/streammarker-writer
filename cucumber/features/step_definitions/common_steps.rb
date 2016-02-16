Given(/^I have a clean database$/) do
  begin
    teardown_tables
  rescue Exception => e
    # ignore    
  end

  setup_tables
end

Then(/^the result should be a (\d+)$/) do |stat_code|
  # We _really_ shouldn't need code.to_i, but when @response has been pulled out of an exception, @response.code
  # becomes a string. Sigh.
  if @response.code.to_i != stat_code.to_i
    puts @response.body
  end
  @response.code.to_i.should eq stat_code.to_i
end

Given(/^sleep (\d+)$/) do |seconds|
  sleep seconds.to_i
end

Given(/^I have a non\-JSON payload$/) do
  @request = "This ain't JSON ;"
end

When(/^I call POST to "(.*?)"$/) do |path|
  manipulated_path = path
  if manipulated_path.include? 'TITLEID'
    manipulated_path = manipulated_path.sub 'TITLEID', @title['id']
  end
  @response = HTTParty.post(APPLICATION_ENDPOINT + "#{manipulated_path}",
                            body: @request,
                            headers: { 'Content-Type' => 'application/json',
                                       'Accept' => 'application/json',
                                       'X-Fabric-Token' => '1e98db83-22b2-4bf9-b637-abe57c62610e'})
end

Then(/^the json response should match "(.*)"$/) do |name|
  expected = get_response(name)
  @response.body.should be_json_eql(expected).excluding('id', 'title_id', 'url')
end

Then(/^the response should have an? "(.*?)"$/) do |field|
  @response.body.should have_json_path(field)
end

Then(/^the response should have an? "(.*)" of type String$/) do |field|
  expect(@response.body).to have_json_type(String).at_path(field)
end

Then(/^the "(.*?)" timestamp should be within (\d+) seconds?$/) do |field, allowed|
  json = JSON.parse(@response.body)
  begin
    parsed = Time.iso8601(json[field].to_s)
  rescue
    fail(field + ' was not ISO8601 compliant. Got: ' + json[field].to_s)
  end
  diff = (Time.new - parsed)
  diff.should be < allowed.to_i
end


When(/^I call POST to "(.*?)" with no token$/) do |path|
  manipulated_path = path
  if manipulated_path.include? 'TITLEID'
    manipulated_path = manipulated_path.sub 'TITLEID', @title['id']
  end
  @response = HTTParty.post(APPLICATION_ENDPOINT + "#{manipulated_path}",
                            body: @request,
                            headers: { 'Content-Type' => 'application/json',
                                       'Accept' => 'application/json'})
end

When(/^I call POST to "(.*?)" with an incorrect token$/) do |path|
  manipulated_path = path
  if manipulated_path.include? 'TITLEID'
    manipulated_path = manipulated_path.sub 'TITLEID', @title['id']
  end
  @response = HTTParty.post(APPLICATION_ENDPOINT + "#{manipulated_path}",
                            body: @request,
                            headers: { 'Content-Type' => 'application/json',
                                       'Accept' => 'application/json',
                                       'X-Fabric-Token' => 'wrongtoken'})
end

When(/^I call POST to "(.*?)" with too many tokens$/) do |path|
  manipulated_path = path
  if manipulated_path.include? 'TITLEID'
    manipulated_path = manipulated_path.sub 'TITLEID', @title['id']
  end
  @response = HTTParty.post(APPLICATION_ENDPOINT + "#{manipulated_path}",
                            body: @request,
                            headers: { 'Content-Type' => 'application/json',
                                       'Accept' => 'application/json',
                                       'X-Fabric-Token' => '1e98db83-22b2-4bf9-b637-abe57c62610e, extratoken'})
end