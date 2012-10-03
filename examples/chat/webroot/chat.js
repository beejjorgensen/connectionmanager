(function() {

// TODO
// set/show username
// show user list

var userInfo = {};
var userList = []; // users on the chat
var logger;
var failMessagePosted = false;

// set up a logger
if (typeof console == "undefined" || typeof console.log == "undefined") {
	logger = function() { };
} else {
	logger = function(m) { console.log(m); }
}

/**
 * Fix &<>" in HTML string
 */
function escapeHTML(s) {
	return s.replace(/&/g, "&amp;").replace(/>/g, "&gt;").replace(/</g, "&lt;").replace(/"/g, "&quot;");
}

/**
 * Do something later
 */
function defer(f)
{
	setTimeout(f, 5);
}

/**
 * make random ID (UUID-ish), stolen from stackoverflow
 */
function randomID()
{
	var S4 = function() {
		return Math.floor(Math.random() * 0x10000).toString(16);
	};

	return (
			S4() + S4() + "-" +
			S4() + "-" +
			S4() + "-" +
			S4() + "-" +
			S4() + S4() + S4()
		);
}

/**
 * Add a message to the chat window
 */
function addChatMessage(username, msg) {
	var html = "<p>";
	var msgclass;
	
	if (username) {
		html += '<span class="chat-username">' + username + '</span>';
		html += '<span class="chat-sep">:</span>&nbsp;';
		msgclass = 'chat-message';
	} else {
		msgclass = 'system-message';
	}

	html += '<span class="' + msgclass + '">' + msg + '</span></p>';

	// append HTML and scroll down
	$('#output-div').append(html);

	/*
	defer(function() {
		$('#output-div').animate({'scrollTop': $("#output-div").get(0).scrollHeight}, 500);
	});
	*/
	$('#output-div').get(0).scrollTop = $("#output-div").get(0).scrollHeight;
}

/**
 * Handle a long poll message
 */
function handleMessage(m) {

	switch(m.type) {
		case "message":
			addChatMessage(m.username, escapeHTML(m.message));
			break;

		case "newuser":
			userList.push({
				"publicid": m.publicid,
				"username": m.username
			});
			addChatMessage(null, m.username + " joined the chat");
			break;

		default:
			logger("unknown message type: " + m.type);
	}
}

/**
 * Run a long poll
 */
function longPoll() {
	logger("longpoll: start");

	function longPollSuccess(data, textStatus, jqXHR) {
		failMessagePosted = false

		logger("longpoll: got result");

		if (data.type == "status" && data.status == "error") {
			logger("longpoll error: " + data.message)
			addChatMessage(null, "Long poll error: " + data.message);
			setTimeout(function() { longPoll(); }, 5000);
			return
		}

		// process
		for (var i = 0, l = data.length; i < l; i++) {
			handleMessage(data[i])
		}

		// again!
		longPoll();
	}

	function longPollError(jqXHR, textStatus, errorThrown) {
		if (textStatus == "error") {
			logger("longpoll error: " + textStatus + " " + errorThrown)

			if (!failMessagePosted) {
				addChatMessage(null, "Error polling server");
				failMessagePosted = true
			}

			// again, after a delay
			setTimeout(function() { longPoll(); }, 5000);
		} else {
			logger("longpoll complete");

			// error is timeout or abort
			longPoll();
		}
	}

	$.ajax({
		"cache": false,
		"data": { "id": userInfo.id },
		"timeout": 120*1000,
		"url": "poll",
		"type": "POST",
		"success": longPollSuccess,
		"error": longPollError
	});
}

/**
 * Send generic command to server
 *
 * @param data object with data to send
 * @param success callback for success
 * @param error callback for error
 */
function sendCommand(data, success, error) {
	$.ajax({
		"cache": false,
		"data": data,
		"url": "cmd",
		"type": "POST",
		"success": success,
		"error": error
	});
}

/**
 * Send text to the server
 */
function sendText() {
	function success(data, textStatus, jqXHR) {
		if (data.type == "status" && data.status == "error") {
			addChatMessage(null, "Error sending data to server: " + data.message);
		}
		$('#input-field').val('');
	}

	function error(jqXHR, textStatus, errorThrown) {
		addChatMessage(null, "Error sending data to server");
	}

	var text = $('#input-field').val();
	text = $.trim(text);

	if (text == '') { return; }

	sendCommand({
			"command": "broadcast",
			"id": userInfo.id,
			"message": text
		}, success, error);
}


/**
 * Look for returns on the input form
 */
function inputKeyPressed(ev) {
	if (ev.which == 13) {
		sendText();
		return false;
	}
}

/**
 * Do a login
 */
function login(completeCallback) {
	function success(data, textStatus, jqXHR) {
		if (data.type == "status" && data.status == "error") {
			addChatMessage(null, "Error logging in: " + data.message);
			return
		}

		// get response
		userInfo.pubId = data.publicid;
		userInfo.username = data.username;

		// TODO: get user list in separate request
		//userList = data.userlist;

		if (completeCallback) {
			addChatMessage(null, "Welcome to the chat, " + userInfo.username);

			completeCallback();
		}
	}

	function error(jqXHR, textStatus, errorThrown) {
		logger("login error: " + textStatus);
	}

	userInfo.id = randomID();

	// TODO: set up with a username ahead of time
	sendCommand({
			"command": "login",
			"id": userInfo.id
		}, success, error);
}

/**
 * Stuff to do once login is complete
 */
function onLoginComplete() {
	longPoll();
	$('#send-button').on('click', sendText);
	$('#input-field').on('keypress', inputKeyPressed);
}

/**
 * On ready
 */
$(function() {
	login(onLoginComplete)
});

})();
