
const apiFactory = () => {
    const baseUrl = "http://localhost:8080/v1";
    let token = "";

    const headers = () => ({
        'Authorization': 'Bearer ' + token
    });

    const checkLogin = async () => {
        const token = localStorage.getItem("token");

        if (token) {
            const isLoggedIn = await logIn(token);
            return isLoggedIn;
        }
        return false;
    };

    const logIn = async (password, remember) => {
        token = password;
        const response = await fetch(baseUrl + "/list", {
            credentials: 'include',
            headers: headers()
        });

        if (response.ok) {
            if (remember === true) {
                localStorage.setItem("token", password);
            }
            return true;
        }

        token = "";
        return false;
    };

    const logOut = () => {
        localStorage.clear();
    };

    const list = async () => {
        const response = await fetch(baseUrl + "/list", {
            credentials: 'include',
            headers: headers()
        });
        const data = await response.json();
        return data;
    };

    const check = async (containerId) => {
        const requestData = {
            ContainerId: containerId
        };
        const response = await fetch(baseUrl + "/check", {
            method: 'POST',
            credentials: 'include',
            headers: {
                ...headers(),
                'Content-Type': 'application/json',
            },
            body: JSON.stringify(requestData)
        });
        const data = await response.json();
        return data;
    };

    const update = async (images) => {
        let updateUrl = new URL(baseUrl + "/update");

        if (images instanceof Array) {
            images.map(image => updateUrl.searchParams.append("image", image));
        }

        const response = await fetch(updateUrl.toString(), {
            credentials: 'include',
            headers: headers(),
        });

        return response.ok;
    };

    return {
        checkLogin,
        logIn,
        logOut,
        list,
        check,
        update
    }
};