
var clearTweets = function(){
    content = document.getElementById("content");
    content.innerHTML = '';
}

var formatTime = function(unix_timestamp) {
    // Create a new JavaScript Date object based on the timestamp (nanoseconds)
    t = parseInt("0x"+unix_timestamp);
    var date = new Date(t/1000000);
    return date.toLocaleString();
}

var buildTweetDiv = function(userid, contents, timestamp) {
    var postkey = userid+":"+"post_"+timestamp;
    var div = document.createElement("div");
    div.setAttribute("class", "tweet");
    var p1 = document.createElement("p");
    var node = document.createTextNode("@"+userid);
    p1.appendChild(node);
    var p2 = document.createElement("p");
    // node = document.createTextNode(contents);
    // p2.appendChild(node);
    p2.innerHTML = contents;
    div.appendChild(p1);
    div.appendChild(p2);
    

    var div2 = document.createElement("div");
    var bu1 = document.createElement("button");
    bu1.onclick = subscribe
    bu1.innerHTML = 'subscribe';
    bu1.id = postkey+".a"
    var bu2 = document.createElement("button");
    bu2.onclick = unsubscribe
    bu2.innerHTML = 'unsubscribe';
    bu2.id = postkey+".b"
    var bu3 = document.createElement("button");
    bu3.onclick = deleteTweet
    bu3.innerHTML = 'delete';
    bu3.id = postkey+".c"
    var p3 = document.createElement("p");
    p3.setAttribute("style", "font-size:12px")
    node = document.createTextNode("Posted at: "+formatTime(timestamp));
    p3.appendChild(node);
    div2.appendChild(p3);
    div2.appendChild(bu1);
    div2.appendChild(bu2);
    div2.appendChild(bu3);
    div.appendChild(div2);


    return div
}
var appendTweet = function(userid, contents, timestamp) {
    content = document.getElementById("content");
    content.appendChild(buildTweetDiv(userid, contents, timestamp));
}

var appendTweetFront = function(userid, contents, timestamp) {
    content = document.getElementById("content");
    content.insertBefore(buildTweetDiv(userid, contents, timestamp), content.firstChild);
}

var postTweet = function(){
    var xhr = new XMLHttpRequest();
    var url = "/posts";
    xhr.open("POST", url, true);
    xhr.setRequestHeader("Content-Type", "application/json");
    xhr.onreadystatechange = function () {
        if (xhr.readyState === 4 && xhr.status === 200) {
            console.log(xhr.responseText);
            flushContent();
        }
    };
    var data = JSON.stringify({
        "UserID": document.getElementById("post_UserID").value,
        "Contents": document.getElementById("post_Contents").value
    });
    xhr.send(data);
};

var deleteTweet = function() {
    var postKey = this.id.split(".")[0];
    var uid = document.getElementById("post_UserID").value
    var xhr = new XMLHttpRequest();
    var url = "/posts?UserID="+uid+"&PostKey="+postKey;
    xhr.open("DELETE", url, true);
    xhr.setRequestHeader("Content-Type", "application/json");
    xhr.onreadystatechange = function () {
        if (xhr.readyState === 4 && xhr.status === 200) {
            console.log(xhr.responseText);
            flushContent();
        }
    };
    xhr.send();
};

var subscribe = function() {
    var t = this.id.split(":")[0];
    var s = document.getElementById("post_UserID").value
    var xhr = new XMLHttpRequest();
    var url = "/subscriptions?UserID="+s+"&TargetUserID="+t;
    xhr.open("POST", url, true);
    xhr.setRequestHeader("Content-Type", "application/json");
    xhr.onreadystatechange = function () {
        if (xhr.readyState === 4 && xhr.status === 200) {
            console.log(xhr.responseText);
        }
        flushContent();
    };
    xhr.send();
};

var unsubscribe = function(id) {
    var t = this.id.split(":")[0];
    var s = document.getElementById("post_UserID").value
    var xhr = new XMLHttpRequest();
    var url = "/subscriptions?UserID="+s+"&TargetUserID="+t;
    xhr.open("DELETE", url, true);
    xhr.setRequestHeader("Content-Type", "application/json");
    xhr.onreadystatechange = function () {
        if (xhr.readyState === 4 && xhr.status === 200) {
            console.log(xhr.responseText);
            flushContent();
        }
    };
    xhr.send();
};

var flushContent = function() {
    var page = "Home"
    var naviButtons = document.getElementById("navigation_bar").childNodes
    for (i=0; i<naviButtons.length; i++){
        e = naviButtons[i];
        if(e.className == "button_on"){
            page = e.firstChild.nodeValue.trim();
            break;
        }
    }

    var url = "";
    if (page == "Home") {
        uid = document.getElementById("post_UserID").value;
        url = "/home?UserID="+uid;
    } else if (page == "MyTimeline") {
        uid = document.getElementById("post_UserID").value;
        url = "/timeline?UserID="+uid;
    } else if (page == "Discover") {
        uid = document.getElementById("post_UserID").value;
        url = "/home?UserID="+uid;
    }

    var xhr = new XMLHttpRequest();
    xhr.open("GET", url, true);
    xhr.setRequestHeader("Content-Type", "application/json");
    appendTweetFront("Loading...", "Loading...", "0")
    xhr.onreadystatechange = function () {
        if (xhr.readyState === 4 && xhr.status === 200) {
            clearTweets();
            //console.log(xhr.responseText);
            var json = JSON.parse(xhr.responseText);
            var posts = json.Posts;
            for (i=0; i<posts.length; i++){
                appendTweet(posts[i].UserID, posts[i].Contents, posts[i].Posted);
            }
        }
    };
    xhr.send();
}

var displayPage = function(page) {
    var naviButtons = document.getElementById("navigation_bar").childNodes
    for (i=0; i<naviButtons.length; i++){
        e = naviButtons[i];
        if(e.className == "button" || e.className == "button_on"){
            if (e.firstChild.nodeValue.trim() == page) {
                e.setAttribute("class", "button_on");
            } else {
                e.setAttribute("class", "button");
            }
        }
    }
    flushContent();
}

var switchUser = function() {
    flushContent()
}

var displayHome = function() {
    displayPage("Home");
};

var displayMyTimeline = function() {
    displayPage("MyTimeline");
}

var main = function() {
    flushContent();
}

main()